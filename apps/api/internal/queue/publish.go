package queue

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"jiaohao/internal/canteen"
)

func (s *Service) publishTicket(ctx context.Context, ticket Ticket, announcementID string) {
	s.publishWindow(ctx, ticket.WindowID)
	if s.pub == nil {
		return
	}
	topics := []string{"window:" + ticket.WindowID}
	if ticket.OwnerID != "" {
		topics = append(topics, "ticket:"+ticket.OwnerID)
	}
	s.pub.Publish(topics, map[string]any{
		"event":  "queue.ticket.updated",
		"ticket": ticket,
	})
	if announcementID != "" {
		s.pub.Publish(topics, map[string]any{
			"event":           "queue.announcement",
			"announcement_id": announcementID,
			"window_id":       ticket.WindowID,
			"window_name":     ticket.WindowName,
			"number":          ticket.Number,
			"text":            "请 " + strconv.Itoa(ticket.Number) + " 号到 " + ticket.WindowName,
		})
	}
}

func (s *Service) publishWindow(ctx context.Context, windowID string) {
	if s.pub == nil {
		return
	}
	id, err := uuid.Parse(windowID)
	if err != nil {
		return
	}
	var status string
	var current, waiting int
	day := canteen.BusinessDate(time.Now(), s.loc)
	err = s.pool.QueryRow(ctx, `
		SELECT w.status,
		       COALESCE((
		           SELECT t.number FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'called'
		           ORDER BY t.called_at DESC NULLS LAST LIMIT 1
		       ), 0),
		       (
		           SELECT COUNT(*) FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'waiting'
		       )
		FROM windows w WHERE w.id = $1
	`, id, day).Scan(&status, &current, &waiting)
	if err != nil {
		return
	}
	s.pub.Publish([]string{"window:" + windowID}, map[string]any{
		"event":          "queue.window.updated",
		"window_id":      windowID,
		"current_number": current,
		"waiting_count":  waiting,
		"status":         status,
	})
}
