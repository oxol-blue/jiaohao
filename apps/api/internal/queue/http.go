package queue

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
)

type Handler struct {
	svc *Service
	log *logx.Logger
}

func NewHandler(svc *Service, log *logx.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

type takeRequest struct {
	WindowID string `json:"window_id"`
}

func (h *Handler) Take(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	var req takeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "请求格式不正确")
		return
	}
	windowID, err := uuid.Parse(req.WindowID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "window_id 不正确")
		return
	}
	ticket, err := h.svc.Take(r.Context(), user, windowID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, ticket)
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	ticket, err := h.svc.Mine(r.Context(), user)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, ticket)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "票号 ID 不正确")
		return
	}
	ticket, err := h.svc.Cancel(r.Context(), user, id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, ticket)
}

func (h *Handler) StaffWindows(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	items, err := h.svc.StaffWindows(r.Context(), user)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) windowAction(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, user identity.User, windowID uuid.UUID) (Ticket, error)) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	ticket, err := fn(r.Context(), user, id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, ticket)
}

func (h *Handler) CallNext(w http.ResponseWriter, r *http.Request) {
	h.windowAction(w, r, h.svc.CallNext)
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	h.windowAction(w, r, h.svc.Complete)
}

func (h *Handler) Skip(w http.ResponseWriter, r *http.Request) {
	h.windowAction(w, r, h.svc.Skip)
}

func (h *Handler) PauseTake(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, true)
}

func (h *Handler) ResumeTake(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, false)
}

func (h *Handler) CloseWindow(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "closed")
}

func (h *Handler) OpenWindow(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "open")
}

func (h *Handler) IssueDisplayToken(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	token, err := h.svc.IssueDisplayToken(r.Context(), user, id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{
		"token": token,
		"path":  "/display/" + id.String() + "?token=" + token,
	})
}

func (h *Handler) RevokeDisplayToken(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	if err := h.svc.RevokeDisplayToken(r.Context(), user, id); err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"revoked": true})
}

func (h *Handler) Display(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Display-Token")
	}
	view, err := h.svc.DisplayView(r.Context(), id, token)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, view)
}

func (h *Handler) setPaused(w http.ResponseWriter, r *http.Request, paused bool) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	if err := h.svc.SetTakePaused(r.Context(), user, id, paused); err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"paused": paused})
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	user, ok := identity.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	if err := h.svc.SetWindowStatus(r.Context(), user, id, status); err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"status": status})
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	status, code, msg := APIStatus(err)
	if status >= 500 {
		h.log.Error("queue.error", map[string]any{
			"request_id": httpx.RequestIDFrom(r.Context()),
			"outcome":    "error",
			"error":      err.Error(),
		})
	}
	httpx.WriteError(w, status, code, msg)
}
