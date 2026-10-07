package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")

const (
	RoleDiner = "diner"
	RoleStaff = "staff"
	RoleAdmin = "admin"
)

type User struct {
	ID           uuid.UUID
	StudentID    string
	PasswordHash string
	DisplayName  string
	Role         string
	Status       string
	TokenVersion int
	CreatedAt    time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetByStudentID(ctx context.Context, studentID string) (User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, student_id, password_hash, display_name, role, status, token_version, created_at
		FROM users
		WHERE student_id = $1
	`, studentID))
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, student_id, password_hash, display_name, role, status, token_version, created_at
		FROM users
		WHERE id = $1
	`, id))
}

func (s *Store) ExistsByStudentID(ctx context.Context, studentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE student_id = $1)`, studentID).Scan(&exists)
	return exists, err
}

func (s *Store) Create(ctx context.Context, studentID, passwordHash, displayName, role string) (User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (student_id, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, student_id, password_hash, display_name, role, status, token_version, created_at
	`, studentID, passwordHash, displayName, role))
}

func (s *Store) BumpTokenVersion(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET token_version = token_version + 1, updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.StudentID, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.TokenVersion, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
