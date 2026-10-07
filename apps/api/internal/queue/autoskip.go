package queue

import (
	"context"
	"time"

	"github.com/google/uuid"

	"jiaohao/internal/logx"
)

func (s *Service) AutoSkipDue(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE tickets
		SET status = 'skipped', finished_at = now()
		WHERE status = 'called'
		  AND skip_after IS NOT NULL
		  AND skip_after <= now()
		RETURNING id
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		ticket, err := s.GetByID(ctx, id)
		if err != nil {
			continue
		}
		s.log.Info("queue.auto_skip", map[string]any{
			"ticket_id": ticket.ID,
			"window_id": ticket.WindowID,
			"outcome":   "ok",
		})
		s.publishTicket(ctx, ticket, "")
	}
	return len(ids), nil
}

func StartAutoSkip(ctx context.Context, svc *Service, log *logx.Logger, every time.Duration) {
	if every <= 0 {
		every = time.Second
	}
	t := time.NewTicker(every)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n, err := svc.AutoSkipDue(ctx)
				if err != nil {
					log.Error("queue.auto_skip", map[string]any{"outcome": "error", "error": err.Error()})
					continue
				}
				if n > 0 {
					log.Info("queue.auto_skip_batch", map[string]any{"count": n, "outcome": "ok"})
				}
			}
		}
	}()
}
