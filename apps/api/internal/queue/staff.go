package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"jiaohao/internal/canteen"
	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
)

type StaffWindow struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	Status        string    `json:"status"`
	CanteenID     uuid.UUID `json:"canteen_id"`
	CurrentNumber int       `json:"current_number"`
	WaitingCount  int       `json:"waiting_count"`
	CalledTicket  *Ticket   `json:"called_ticket"`
}

func (s *Service) requireStaffGrant(ctx context.Context, tx pgx.Tx, user identity.User, windowID uuid.UUID) error {
	if user.Role != identity.RoleStaff {
		return ErrForbidden
	}
	q := s.pool
	var exists bool
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM staff_window_grants WHERE user_id = $1 AND window_id = $2)
		`, user.ID, windowID).Scan(&exists)
	} else {
		err = q.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM staff_window_grants WHERE user_id = $1 AND window_id = $2)
		`, user.ID, windowID).Scan(&exists)
	}
	if err != nil {
		return err
	}
	if !exists {
		return ErrForbidden
	}
	return nil
}

func (s *Service) StaffWindows(ctx context.Context, user identity.User) ([]StaffWindow, error) {
	if user.Role != identity.RoleStaff {
		return nil, ErrForbidden
	}
	day := canteen.BusinessDate(time.Now(), s.loc)
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.name, w.code, w.status, w.canteen_id,
		       COALESCE((
		           SELECT t.number FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'called'
		           ORDER BY t.called_at DESC NULLS LAST LIMIT 1
		       ), 0) AS current_number,
		       (
		           SELECT COUNT(*) FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'waiting'
		       ) AS waiting_count
		FROM windows w
		JOIN staff_window_grants g ON g.window_id = w.id
		WHERE g.user_id = $1
		ORDER BY w.sort, w.code
	`, user.ID, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StaffWindow, 0)
	for rows.Next() {
		var w StaffWindow
		if err := rows.Scan(&w.ID, &w.Name, &w.Code, &w.Status, &w.CanteenID, &w.CurrentNumber, &w.WaitingCount); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Service) CallNext(ctx context.Context, user identity.User, windowID uuid.UUID) (Ticket, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Ticket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.requireStaffGrant(ctx, tx, user, windowID); err != nil {
		return Ticket{}, err
	}

	day := canteen.BusinessDate(time.Now(), s.loc)
	var calledID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM tickets
		WHERE window_id = $1 AND business_date = $2 AND status = 'called'
		FOR UPDATE
	`, windowID, day).Scan(&calledID)
	if err == nil {
		return Ticket{}, ErrCalledPending
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, err
	}

	var timeout int
	if err := tx.QueryRow(ctx, `
		SELECT skip_timeout_seconds FROM windows WHERE id = $1 FOR SHARE
	`, windowID).Scan(&timeout); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Ticket{}, ErrWindowNotFound
		}
		return Ticket{}, err
	}

	var ticketID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM tickets
		WHERE window_id = $1 AND business_date = $2 AND status = 'waiting'
		ORDER BY number
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, windowID, day).Scan(&ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrNoWaiting
	}
	if err != nil {
		return Ticket{}, err
	}

	announcementID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		UPDATE tickets
		SET status = 'called',
		    called_at = now(),
		    skip_after = CASE WHEN $2::int > 0 THEN now() + make_interval(secs => $2::int) ELSE NULL END,
		    announcement_id = $3
		WHERE id = $1 AND status = 'waiting'
	`, ticketID, timeout, announcementID); err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Ticket{}, err
	}
	ticket, err := s.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	s.log.Info("queue.call_next", map[string]any{
		"request_id":      httpx.RequestIDFrom(ctx),
		"user_id":         user.ID.String(),
		"window_id":       windowID.String(),
		"ticket_id":       ticketID.String(),
		"announcement_id": announcementID,
		"outcome":         "ok",
	})
	s.publishTicket(ctx, ticket, announcementID)
	return ticket, nil
}

func (s *Service) finishCalled(ctx context.Context, user identity.User, windowID uuid.UUID, nextStatus, event string) (Ticket, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Ticket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.requireStaffGrant(ctx, tx, user, windowID); err != nil {
		return Ticket{}, err
	}
	day := canteen.BusinessDate(time.Now(), s.loc)
	var ticketID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM tickets
		WHERE window_id = $1 AND business_date = $2 AND status = 'called'
		FOR UPDATE
	`, windowID, day).Scan(&ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrTicketNotCalled
	}
	if err != nil {
		return Ticket{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tickets SET status = $2, finished_at = now()
		WHERE id = $1 AND status = 'called'
	`, ticketID, nextStatus); err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Ticket{}, err
	}
	s.log.Info(event, map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
		"window_id":  windowID.String(),
		"ticket_id":  ticketID.String(),
		"outcome":    "ok",
	})
	ticket, err := s.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	s.publishTicket(ctx, ticket, "")
	return ticket, nil
}

func (s *Service) Complete(ctx context.Context, user identity.User, windowID uuid.UUID) (Ticket, error) {
	return s.finishCalled(ctx, user, windowID, "completed", "queue.complete")
}

func (s *Service) Skip(ctx context.Context, user identity.User, windowID uuid.UUID) (Ticket, error) {
	return s.finishCalled(ctx, user, windowID, "skipped", "queue.skip")
}

func (s *Service) SetTakePaused(ctx context.Context, user identity.User, windowID uuid.UUID, paused bool) error {
	if err := s.requireStaffGrant(ctx, nil, user, windowID); err != nil {
		return err
	}
	status := "open"
	if paused {
		status = "pause_take"
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE windows SET status = $2, updated_at = now()
		WHERE id = $1 AND status IN ('open', 'pause_take')
	`, windowID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWindowNotFound
	}
	s.log.Info("queue.window_status", map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
		"window_id":  windowID.String(),
		"status":     status,
		"outcome":    "ok",
	})
	s.publishWindow(ctx, windowID.String())
	return nil
}
