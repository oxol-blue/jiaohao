package queue

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"jiaohao/internal/canteen"
	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
)

var ErrDisplayToken = errors.New("display token invalid")

type DisplayView struct {
	WindowID       string `json:"window_id"`
	WindowName     string `json:"window_name"`
	Code           string `json:"code"`
	Status         string `json:"status"`
	CurrentNumber  int    `json:"current_number"`
	WaitingCount   int    `json:"waiting_count"`
	AnnouncementID string `json:"announcement_id"`
}

func hashDisplayToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Service) IssueDisplayToken(ctx context.Context, user identity.User, windowID uuid.UUID) (string, error) {
	if err := s.requireStaffGrant(ctx, nil, user, windowID); err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	tag, err := s.pool.Exec(ctx, `
		UPDATE windows SET display_token_hash = $2, updated_at = now() WHERE id = $1
	`, windowID, hashDisplayToken(token))
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrWindowNotFound
	}
	s.log.Info("queue.display_token", map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
		"window_id":  windowID.String(),
		"outcome":    "issued",
	})
	return token, nil
}

func (s *Service) RevokeDisplayToken(ctx context.Context, user identity.User, windowID uuid.UUID) error {
	if err := s.requireStaffGrant(ctx, nil, user, windowID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE windows SET display_token_hash = NULL, updated_at = now() WHERE id = $1
	`, windowID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWindowNotFound
	}
	s.log.Info("queue.display_token", map[string]any{
		"request_id": httpx.RequestIDFrom(ctx),
		"user_id":    user.ID.String(),
		"window_id":  windowID.String(),
		"outcome":    "revoked",
	})
	return nil
}

func (s *Service) WindowForDisplayToken(ctx context.Context, raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	want := hashDisplayToken(raw)
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, display_token_hash
		FROM windows
		WHERE display_token_hash IS NOT NULL
	`)
	if err != nil {
		return "", false
	}
	defer rows.Close()
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return "", false
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(want)) == 1 {
			return id, true
		}
	}
	return "", false
}

func (s *Service) DisplayView(ctx context.Context, windowID uuid.UUID, rawToken string) (DisplayView, error) {
	bound, ok := s.WindowForDisplayToken(ctx, rawToken)
	if !ok || bound != windowID.String() {
		return DisplayView{}, ErrDisplayToken
	}
	day := canteen.BusinessDate(time.Now(), s.loc)
	var view DisplayView
	var announcement *string
	err := s.pool.QueryRow(ctx, `
		SELECT w.id::text, w.name, w.code, w.status,
		       COALESCE((
		           SELECT t.number FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'called'
		           ORDER BY t.called_at DESC NULLS LAST LIMIT 1
		       ), 0),
		       (
		           SELECT COUNT(*) FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'waiting'
		       ),
		       (
		           SELECT t.announcement_id FROM tickets t
		           WHERE t.window_id = w.id AND t.business_date = $2 AND t.status = 'called'
		           ORDER BY t.called_at DESC NULLS LAST LIMIT 1
		       )
		FROM windows w WHERE w.id = $1
	`, windowID, day).Scan(&view.WindowID, &view.WindowName, &view.Code, &view.Status, &view.CurrentNumber, &view.WaitingCount, &announcement)
	if errors.Is(err, pgx.ErrNoRows) {
		return DisplayView{}, ErrWindowNotFound
	}
	if err != nil {
		return DisplayView{}, err
	}
	if announcement != nil {
		view.AnnouncementID = *announcement
	}
	return view, nil
}
