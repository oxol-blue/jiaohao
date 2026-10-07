package canteen

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"jiaohao/internal/httpx"
	"jiaohao/internal/logx"
)

type Handler struct {
	store *Store
	loc   *time.Location
	log   *logx.Logger
}

func NewHandler(store *Store, loc *time.Location, log *logx.Logger) *Handler {
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &Handler{store: store, loc: loc, log: log}
}

func BusinessDate(now time.Time, loc *time.Location) time.Time {
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func (h *Handler) ListCanteens(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListCanteens(r.Context())
	if err != nil {
		h.writeStoreErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListFloors(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "食堂 ID 不正确")
		return
	}
	exists, err := h.store.CanteenExists(r.Context(), id)
	if err != nil {
		h.writeStoreErr(w, r, err)
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "食堂不存在")
		return
	}
	items, err := h.store.ListFloors(r.Context(), id)
	if err != nil {
		h.writeStoreErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListWindows(w http.ResponseWriter, r *http.Request) {
	canteenRaw := r.URL.Query().Get("canteen_id")
	canteenID, err := uuid.Parse(canteenRaw)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "canteen_id 必填且须为合法 ID")
		return
	}
	exists, err := h.store.CanteenExists(r.Context(), canteenID)
	if err != nil {
		h.writeStoreErr(w, r, err)
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "食堂不存在")
		return
	}
	var floorID *uuid.UUID
	if raw := r.URL.Query().Get("floor_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "floor_id 不正确")
			return
		}
		floorID = &id
	}
	items, err := h.store.ListWindows(r.Context(), canteenID, floorID, BusinessDate(time.Now(), h.loc))
	if err != nil {
		h.writeStoreErr(w, r, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) writeStoreErr(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("canteen.error", map[string]any{
		"request_id": httpx.RequestIDFrom(r.Context()),
		"outcome":    "error",
		"error":      err.Error(),
	})
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
}
