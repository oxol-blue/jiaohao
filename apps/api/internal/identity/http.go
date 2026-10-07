package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"jiaohao/internal/httpx"
	"jiaohao/internal/logx"
)

type contextKey string

const userContextKey contextKey = "identity.user"

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey).(User)
	return u, ok
}

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

type Handler struct {
	svc *Service
	log *logx.Logger
}

func NewHandler(svc *Service, log *logx.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

type loginRequest struct {
	StudentID string `json:"student_id"`
	Password  string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "请求格式不正确")
		return
	}
	token, user, err := h.svc.Login(r.Context(), req.StudentID, req.Password)
	if err != nil {
		h.writeAuthErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  ToPublic(user),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	if err := h.svc.Logout(r.Context(), user); err != nil {
		h.log.Error("auth.logout", map[string]any{
			"request_id": httpx.RequestIDFrom(r.Context()),
			"user_id":    user.ID.String(),
			"outcome":    "error",
			"error":      err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "退出失败")
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"logged_out": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	httpx.WriteOK(w, http.StatusOK, ToPublic(user))
}

func (h *Handler) AdminPing(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{
		"ok":   true,
		"role": user.Role,
	})
}

func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
			return
		}
		user, err := h.svc.LoadSession(r.Context(), raw)
		if err != nil {
			h.writeAuthErr(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

func (h *Handler) RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFrom(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
				return
			}
			if _, ok := allowed[user.Role]; !ok {
				httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "没有权限")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *Handler) writeAuthErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "学号或密码错误")
	case errors.Is(err, ErrDisabled):
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "账号已停用")
	default:
		h.log.Error("auth.error", map[string]any{
			"request_id": httpx.RequestIDFrom(r.Context()),
			"outcome":    "error",
			"error":      err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	typ, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(typ, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
