package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"jiaohao/internal/canteen"
	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
)

var (
	ErrWindowNotFound   = errors.New("window not found")
	ErrWindowNotOpen    = errors.New("window not open")
	ErrWindowPauseTake  = errors.New("window pause take")
	ErrHasActiveTicket  = errors.New("has active ticket")
	ErrTicketNotFound   = errors.New("ticket not found")
	ErrTicketNotWaiting = errors.New("ticket not waiting")
	ErrTicketNotCalled  = errors.New("ticket not called")
	ErrNoWaiting        = errors.New("no waiting")
	ErrCalledPending    = errors.New("called pending")
	ErrForbidden        = errors.New("forbidden")
)

type Ticket struct {
	ID            string `json:"id"`
	WindowID      string `json:"window_id"`
	CanteenID     string `json:"canteen_id"`
	WindowName    string `json:"window_name"`
	Number        int    `json:"number"`
	Status        string `json:"status"`
	CurrentNumber int    `json:"current_number"`
	PeopleAhead   int    `json:"people_ahead"`
	BusinessDate  string `json:"business_date"`
	OwnerID       string `json:"-"`
}

type Publisher interface {
	Publish(topics []string, payload map[string]any)
}

type Service struct {
	pool *pgxpool.Pool
	loc  *time.Location
	log  *logx.Logger
	pub  Publisher
}

func NewService(pool *pgxpool.Pool, loc *time.Location, log *logx.Logger, pub Publisher) *Service {
	return &Service{pool: pool, loc: loc, log: log, pub: pub}
}

func (s *Service) Take(ctx context.Context, user identity.User, windowID uuid.UUID) (Ticket, error) {
	if user.Role != identity.RoleDiner {
		return Ticket{}, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Ticket{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var win struct {
		ID        uuid.UUID
		CanteenID uuid.UUID
		Name      string
		Status    string
	}
	err = tx.QueryRow(ctx, `
		SELECT id, canteen_id, name, status
		FROM windows
		WHERE id = $1
		FOR SHARE
	`, windowID).Scan(&win.ID, &win.CanteenID, &win.Name, &win.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrWindowNotFound
	}
	if err != nil {
		return Ticket{}, err
	}

	var activeID uuid.UUID
	var activeWindow uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id, window_id
		FROM tickets
		WHERE user_id = $1 AND status IN ('waiting', 'called')
		FOR UPDATE
	`, user.ID).Scan(&activeID, &activeWindow)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, err
	}
	if err == nil {
		if activeWindow == windowID {
			if err := tx.Commit(ctx); err != nil {
				return Ticket{}, err
			}
			return s.GetByID(ctx, activeID)
		}
		return Ticket{}, ErrHasActiveTicket
	}

	switch win.Status {
	case "open":
	case "pause_take":
		return Ticket{}, ErrWindowPauseTake
	default:
		return Ticket{}, ErrWindowNotOpen
	}

	day := canteen.BusinessDate(time.Now(), s.loc)
	if _, err := tx.Exec(ctx, `
		INSERT INTO window_day_counters (window_id, business_date, next_number)
		VALUES ($1, $2, 1)
		ON CONFLICT (window_id, business_date) DO NOTHING
	`, windowID, day); err != nil {
		return Ticket{}, err
	}

	var next int
	if err := tx.QueryRow(ctx, `
		SELECT next_number
		FROM window_day_counters
		WHERE window_id = $1 AND business_date = $2
		FOR UPDATE
	`, windowID, day).Scan(&next); err != nil {
		return Ticket{}, err
	}

	var ticketID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO tickets (window_id, user_id, business_date, number, status)
		VALUES ($1, $2, $3, $4, 'waiting')
		RETURNING id
	`, windowID, user.ID, day, next).Scan(&ticketID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Ticket{}, ErrHasActiveTicket
		}
		return Ticket{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE window_day_counters
		SET next_number = next_number + 1
		WHERE window_id = $1 AND business_date = $2
	`, windowID, day); err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Ticket{}, err
	}

	ticket, err := s.GetByID(ctx, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	s.log.Info("queue.take", map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
		"role":       user.Role,
		"window_id":  windowID.String(),
		"ticket_id":  ticketID.String(),
		"number":     ticket.Number,
		"outcome":    "ok",
	})
	s.publishTicket(ctx, ticket, "")
	return ticket, nil
}

func (s *Service) Mine(ctx context.Context, user identity.User) (*Ticket, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM tickets
		WHERE user_id = $1 AND status IN ('waiting', 'called')
		ORDER BY created_at DESC
		LIMIT 1
	`, user.ID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) Cancel(ctx context.Context, user identity.User, ticketID uuid.UUID) (Ticket, error) {
	var owner uuid.UUID
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, status FROM tickets WHERE id = $1
	`, ticketID).Scan(&owner, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrTicketNotFound
	}
	if err != nil {
		return Ticket{}, err
	}
	if owner != user.ID {
		return Ticket{}, ErrForbidden
	}
	if status != "waiting" {
		return Ticket{}, ErrTicketNotWaiting
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE tickets
		SET status = 'cancelled', finished_at = now(), cancel_reason = 'diner'
		WHERE id = $1 AND user_id = $2 AND status = 'waiting'
	`, ticketID, user.ID)
	if err != nil {
		return Ticket{}, err
	}
	if tag.RowsAffected() == 0 {
		return Ticket{}, ErrTicketNotWaiting
	}
	s.log.Info("queue.cancel", map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
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

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Ticket, error) {
	var t Ticket
	var day time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.window_id, w.canteen_id, w.name, t.number, t.status, t.business_date, t.user_id,
		       COALESCE((
		           SELECT x.number FROM tickets x
		           WHERE x.window_id = t.window_id AND x.business_date = t.business_date AND x.status = 'called'
		           ORDER BY x.called_at DESC NULLS LAST, x.number DESC
		           LIMIT 1
		       ), 0) AS current_number,
		       CASE WHEN t.status = 'waiting' THEN (
		           SELECT COUNT(*) FROM tickets x
		           WHERE x.window_id = t.window_id
		             AND x.business_date = t.business_date
		             AND x.status = 'waiting'
		             AND x.number < t.number
		       ) ELSE 0 END AS people_ahead
		FROM tickets t
		JOIN windows w ON w.id = t.window_id
		WHERE t.id = $1
	`, id).Scan(&t.ID, &t.WindowID, &t.CanteenID, &t.WindowName, &t.Number, &t.Status, &day, &t.OwnerID, &t.CurrentNumber, &t.PeopleAhead)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrTicketNotFound
	}
	if err != nil {
		return Ticket{}, err
	}
	t.BusinessDate = day.Format("2006-01-02")
	return t, nil
}

func APIStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrForbidden):
		return 403, httpx.CodeForbidden, "没有权限"
	case errors.Is(err, ErrWindowNotFound), errors.Is(err, ErrTicketNotFound):
		return 404, httpx.CodeNotFound, "资源不存在"
	case errors.Is(err, ErrWindowNotOpen):
		return 409, httpx.CodeWindowNotOpen, "窗口未营业，不能取号"
	case errors.Is(err, ErrWindowPauseTake):
		return 409, httpx.CodeWindowPauseTake, "窗口已暂停取号"
	case errors.Is(err, ErrHasActiveTicket):
		return 409, httpx.CodeHasActiveTicket, "你已有未完成的号码"
	case errors.Is(err, ErrTicketNotWaiting):
		return 409, httpx.CodeTicketNotWaiting, "只有等待中的号码可以取消"
	case errors.Is(err, ErrTicketNotCalled):
		return 409, httpx.CodeTicketNotCalled, "当前没有已叫号"
	case errors.Is(err, ErrNoWaiting):
		return 409, httpx.CodeNoWaiting, "没有等待中的号码"
	case errors.Is(err, ErrCalledPending):
		return 409, httpx.CodeCalledPending, "请先完成或过号当前号码"
	default:
		return 500, httpx.CodeInternal, "服务内部错误"
	}
}
