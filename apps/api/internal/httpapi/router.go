package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jiaohao/internal/config"
	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
)

func NewRouter(cfg config.Config, pool *pgxpool.Pool, log *logx.Logger) http.Handler {
	store := identity.NewStore(pool)
	svc := identity.NewService(store, cfg.JWTSecret, cfg.JWTTTL, log)
	idHandler := identity.NewHandler(svc, log)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.Recover(log))
	r.Use(httpx.AccessLog(log))
	r.Use(httpx.CORS(cfg.CORSOrigins))
	r.Use(httpx.JSONMaxBytes(1 << 20))

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteOK(w, http.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		if err := pool.Ping(req.Context()); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "数据库不可用")
			return
		}
		httpx.WriteOK(w, http.StatusOK, map[string]any{"status": "ready"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", idHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(idHandler.Auth)
			r.Post("/auth/logout", idHandler.Logout)
			r.Get("/me", idHandler.Me)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Get("/admin/ping", idHandler.AdminPing)
		})
	})

	return r
}
