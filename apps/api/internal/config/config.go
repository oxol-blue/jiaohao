package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIAddr            string
	DatabaseURL        string
	DatabaseSchema     string
	JWTSecret          string
	JWTTTL             time.Duration
	AppTZ              string
	CORSOrigins        []string
	LogLevel           string
	AdminStudentID     string
	AdminPassword      string
	SeedDinerStudentID string
	SeedDinerPassword  string
	SeedStaffStudentID string
	SeedStaffPassword  string
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		APIAddr:            envOr("API_ADDR", ":8080"),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseSchema:     envOr("DATABASE_SCHEMA", "jiaohao"),
		JWTSecret:          strings.TrimSpace(os.Getenv("JWT_SECRET")),
		AppTZ:              envOr("APP_TZ", "Asia/Shanghai"),
		LogLevel:           strings.ToLower(envOr("LOG_LEVEL", "info")),
		AdminStudentID:     envOr("ADMIN_STUDENT_ID", "admin"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
		SeedDinerStudentID: envOr("SEED_DINER_STUDENT_ID", "20260001"),
		SeedDinerPassword:  envOr("SEED_DINER_PASSWORD", "diner123"),
		SeedStaffStudentID: envOr("SEED_STAFF_STUDENT_ID", "staff01"),
		SeedStaffPassword:  envOr("SEED_STAFF_PASSWORD", "staff123"),
	}

	ttlRaw := envOr("JWT_TTL", "12h")
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL: %w", err)
	}
	cfg.JWTTTL = ttl

	origins := envOr("CORS_ORIGINS", "http://localhost:5173")
	for _, part := range strings.Split(origins, ",") {
		if o := strings.TrimSpace(part); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if err := validateSchema(cfg.DatabaseSchema); err != nil {
		return Config{}, err
	}
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-to-a-long-random-string" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set to a non-default value")
	}
	if len(cfg.JWTSecret) < 16 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	return cfg, nil
}

func validateSchema(name string) error {
	if name == "" {
		return fmt.Errorf("DATABASE_SCHEMA is required")
	}
	for i, r := range name {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("DATABASE_SCHEMA %q is not a safe identifier", name)
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	candidates := []string{
		filepath.Join(wd, ".env"),
		filepath.Join(wd, "..", ".env"),
		filepath.Join(wd, "..", "..", ".env"),
	}
	for _, p := range candidates {
		if applyDotEnv(p) {
			return
		}
	}
}

func applyDotEnv(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if q, err := strconv.Unquote(value); err == nil {
				value = q
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return true
}
