package canteen

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("canteen not found")

type Canteen struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Sort int       `json:"sort"`
}

type Floor struct {
	ID        uuid.UUID `json:"id"`
	CanteenID uuid.UUID `json:"canteen_id"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
}

type WindowSummary struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	Status        string    `json:"status"`
	Blurb         string    `json:"blurb"`
	CurrentNumber int       `json:"current_number"`
	WaitingCount  int       `json:"waiting_count"`
	CanteenID     uuid.UUID `json:"canteen_id"`
	FloorID       *string   `json:"floor_id"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListCanteens(ctx context.Context) ([]Canteen, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, sort
		FROM canteens
		ORDER BY sort, name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Canteen, 0)
	for rows.Next() {
		var c Canteen
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CanteenExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM canteens WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (s *Store) CountCanteens(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM canteens`).Scan(&n)
	return n, err
}

func (s *Store) ListFloors(ctx context.Context, canteenID uuid.UUID) ([]Floor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, canteen_id, name, sort
		FROM floors
		WHERE canteen_id = $1
		ORDER BY sort, name, id
	`, canteenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Floor, 0)
	for rows.Next() {
		var f Floor
		if err := rows.Scan(&f.ID, &f.CanteenID, &f.Name, &f.Sort); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) ListWindows(ctx context.Context, canteenID uuid.UUID, floorID *uuid.UUID, businessDate time.Time) ([]WindowSummary, error) {
	query := `
		SELECT w.id, w.name, w.code, w.status, w.blurb, w.canteen_id, w.floor_id,
		       COALESCE((
		           SELECT t.number
		           FROM tickets t
		           WHERE t.window_id = w.id
		             AND t.business_date = $2
		             AND t.status = 'called'
		           ORDER BY t.called_at DESC NULLS LAST, t.number DESC
		           LIMIT 1
		       ), 0) AS current_number,
		       (
		           SELECT COUNT(*)
		           FROM tickets t
		           WHERE t.window_id = w.id
		             AND t.business_date = $2
		             AND t.status = 'waiting'
		       ) AS waiting_count
		FROM windows w
		WHERE w.canteen_id = $1
	`
	args := []any{canteenID, businessDate}
	if floorID != nil {
		query += ` AND w.floor_id = $3`
		args = append(args, *floorID)
	}
	query += ` ORDER BY w.sort, w.code, w.id`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WindowSummary, 0)
	for rows.Next() {
		var w WindowSummary
		var floorID *uuid.UUID
		if err := rows.Scan(&w.ID, &w.Name, &w.Code, &w.Status, &w.Blurb, &w.CanteenID, &floorID, &w.CurrentNumber, &w.WaitingCount); err != nil {
			return nil, err
		}
		if floorID != nil {
			id := floorID.String()
			w.FloorID = &id
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateCanteen(ctx context.Context, name string, sort int) (Canteen, error) {
	var c Canteen
	err := s.pool.QueryRow(ctx, `
		INSERT INTO canteens (name, sort)
		VALUES ($1, $2)
		RETURNING id, name, sort
	`, name, sort).Scan(&c.ID, &c.Name, &c.Sort)
	return c, err
}

func (s *Store) CreateFloor(ctx context.Context, canteenID uuid.UUID, name string, sort int) (Floor, error) {
	var f Floor
	err := s.pool.QueryRow(ctx, `
		INSERT INTO floors (canteen_id, name, sort)
		VALUES ($1, $2, $3)
		RETURNING id, canteen_id, name, sort
	`, canteenID, name, sort).Scan(&f.ID, &f.CanteenID, &f.Name, &f.Sort)
	return f, err
}

func (s *Store) CreateWindow(ctx context.Context, canteenID uuid.UUID, floorID *uuid.UUID, name, code, blurb string, sort int) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO windows (canteen_id, floor_id, name, code, blurb, sort, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'open')
		RETURNING id
	`, canteenID, floorID, name, code, blurb, sort).Scan(&id)
	return id, err
}

func (s *Store) GrantStaffWindow(ctx context.Context, studentID string, windowID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO staff_window_grants (user_id, window_id)
		SELECT id, $2 FROM users WHERE student_id = $1 AND role = 'staff'
		ON CONFLICT DO NOTHING
	`, studentID, windowID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
