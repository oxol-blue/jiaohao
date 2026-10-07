package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"jiaohao/internal/config"
	"jiaohao/internal/logx"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrDisabled           = errors.New("user disabled")
)

type Service struct {
	store  *Store
	secret string
	ttl    time.Duration
	log    *logx.Logger
}

func NewService(store *Store, secret string, ttl time.Duration, log *logx.Logger) *Service {
	return &Service{store: store, secret: secret, ttl: ttl, log: log}
}

type PublicUser struct {
	ID        string `json:"id"`
	StudentID string `json:"student_id"`
	Role      string `json:"role"`
}

func ToPublic(u User) PublicUser {
	return PublicUser{
		ID:        u.ID.String(),
		StudentID: u.StudentID,
		Role:      u.Role,
	}
}

func (s *Service) Login(ctx context.Context, studentID, password string) (string, User, error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" || password == "" {
		return "", User{}, ErrInvalidCredentials
	}
	user, err := s.store.GetByStudentID(ctx, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", User{}, ErrInvalidCredentials
		}
		return "", User{}, err
	}
	if user.Status != "active" {
		return "", User{}, ErrDisabled
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return "", User{}, ErrInvalidCredentials
	}
	token, err := IssueToken(s.secret, s.ttl, user)
	if err != nil {
		return "", User{}, err
	}
	s.log.Info("auth.login", map[string]any{
		"user_id":    user.ID.String(),
		"role":       user.Role,
		"student_id": user.StudentID,
		"outcome":    "ok",
	})
	return token, user, nil
}

func (s *Service) Logout(ctx context.Context, user User) error {
	if err := s.store.BumpTokenVersion(ctx, user.ID); err != nil {
		return err
	}
	s.log.Info("auth.logout", map[string]any{
		"user_id": user.ID.String(),
		"role":    user.Role,
		"outcome": "ok",
	})
	return nil
}

func (s *Service) LoadSession(ctx context.Context, rawToken string) (User, error) {
	claims, err := ParseToken(s.secret, rawToken)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	user, err := s.store.GetByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if user.Status != "active" {
		return User{}, ErrDisabled
	}
	if user.TokenVersion != claims.TokenVersion {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func SeedAccounts(ctx context.Context, store *Store, cfg config.Config, log *logx.Logger) error {
	seeds := []struct {
		studentID string
		password  string
		role      string
		name      string
	}{
		{cfg.AdminStudentID, cfg.AdminPassword, RoleAdmin, "管理员"},
		{cfg.SeedDinerStudentID, cfg.SeedDinerPassword, RoleDiner, "演示用餐者"},
		{cfg.SeedStaffStudentID, cfg.SeedStaffPassword, RoleStaff, "演示员工"},
	}
	for _, seed := range seeds {
		if strings.TrimSpace(seed.studentID) == "" || seed.password == "" || seed.password == "change-me" {
			continue
		}
		exists, err := store.ExistsByStudentID(ctx, seed.studentID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		hash, err := HashPassword(seed.password)
		if err != nil {
			return err
		}
		user, err := store.Create(ctx, seed.studentID, hash, seed.name, seed.role)
		if err != nil {
			return fmt.Errorf("seed %s: %w", seed.role, err)
		}
		log.Info("auth.seed", map[string]any{
			"user_id":    user.ID.String(),
			"role":       user.Role,
			"student_id": user.StudentID,
			"outcome":    "created",
		})
	}
	return nil
}
