package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"jiaohao/internal/httpx"
	"jiaohao/internal/logx"
)

type Handler struct {
	pool *pgxpool.Pool
	log  *logx.Logger
}

func NewHandler(pool *pgxpool.Pool, log *logx.Logger) *Handler {
	return &Handler{pool: pool, log: log}
}

type skipTimeoutReq struct {
	SkipTimeoutSeconds *int `json:"skip_timeout_seconds"`
}

func (h *Handler) PatchWindow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口 ID 不正确")
		return
	}
	var req skipTimeoutReq
	if err := httpx.DecodeJSON(r, &req); err != nil || req.SkipTimeoutSeconds == nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "skip_timeout_seconds 必填")
		return
	}
	if *req.SkipTimeoutSeconds < 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "过号秒数不能为负")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE windows SET skip_timeout_seconds = $2, updated_at = now()
		WHERE id = $1
	`, id, *req.SkipTimeoutSeconds)
	if err != nil {
		h.log.Error("admin.window", map[string]any{"outcome": "error", "error": err.Error()})
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "窗口不存在")
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{
		"id":                   id.String(),
		"skip_timeout_seconds": *req.SkipTimeoutSeconds,
	})
}
