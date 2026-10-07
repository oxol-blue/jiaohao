package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"jiaohao/internal/admin"
	"jiaohao/internal/canteen"
	"jiaohao/internal/config"
	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
	"jiaohao/internal/queue"
	"jiaohao/internal/realtime"
)

func NewRouter(cfg config.Config, pool *pgxpool.Pool, loc *time.Location, log *logx.Logger, qs *queue.Service, hub *realtime.Hub) http.Handler {
	store := identity.NewStore(pool)
	svc := identity.NewService(store, cfg.JWTSecret, cfg.JWTTTL, log)
	idHandler := identity.NewHandler(svc, log)
	canteenHandler := canteen.NewHandler(canteen.NewStore(pool), loc, log)
	queueHandler := queue.NewHandler(qs, log)
	wsHandler := realtime.NewHandler(hub, svc, qs, cfg.CORSOrigins, log)
	adminHandler := admin.NewHandler(pool, log)

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
		r.Get("/ws", wsHandler.ServeHTTP)
		r.Get("/display/windows/{id}", queueHandler.Display)

		r.Group(func(r chi.Router) {
			r.Use(idHandler.Auth)
			r.Post("/auth/logout", idHandler.Logout)
			r.Get("/me", idHandler.Me)
			r.Get("/canteens", canteenHandler.ListCanteens)
			r.Get("/canteens/{id}/floors", canteenHandler.ListFloors)
			r.Get("/windows", canteenHandler.ListWindows)
			r.Post("/tickets", queueHandler.Take)
			r.Get("/me/ticket", queueHandler.Mine)
			r.Post("/tickets/{id}/cancel", queueHandler.Cancel)
			r.Get("/staff/windows", queueHandler.StaffWindows)
			r.Post("/windows/{id}/call-next", queueHandler.CallNext)
			r.Post("/windows/{id}/complete", queueHandler.Complete)
			r.Post("/windows/{id}/skip", queueHandler.Skip)
			r.Post("/windows/{id}/pause-take", queueHandler.PauseTake)
			r.Post("/windows/{id}/resume-take", queueHandler.ResumeTake)
			r.Post("/windows/{id}/close", queueHandler.CloseWindow)
			r.Post("/windows/{id}/open", queueHandler.OpenWindow)
			r.Post("/windows/{id}/display-token", queueHandler.IssueDisplayToken)
			r.Delete("/windows/{id}/display-token", queueHandler.RevokeDisplayToken)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Get("/admin/ping", idHandler.AdminPing)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Patch("/admin/windows/{id}", adminHandler.PatchWindow)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/canteens", adminHandler.CreateCanteen)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/canteens/{id}/floors", adminHandler.CreateFloor)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/windows", adminHandler.CreateWindow)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/grants", adminHandler.Grant)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/users/import", adminHandler.ImportUsers)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Post("/admin/users/import-file", adminHandler.ImportUsersFile)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Patch("/admin/canteens/{id}", adminHandler.RenameCanteen)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Delete("/admin/canteens/{id}", adminHandler.DeleteCanteen)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Patch("/admin/floors/{id}", adminHandler.RenameFloor)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Delete("/admin/floors/{id}", adminHandler.DeleteFloor)
			r.With(idHandler.RequireRole(identity.RoleAdmin)).Delete("/admin/windows/{id}", adminHandler.DeleteWindow)
		})
	})

	return r
}
