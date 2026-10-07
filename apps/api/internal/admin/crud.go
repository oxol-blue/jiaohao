package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
)

type nameReq struct {
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

type windowReq struct {
	CanteenID string `json:"canteen_id"`
	FloorID   string `json:"floor_id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Blurb     string `json:"blurb"`
	Sort      int    `json:"sort"`
}

type grantReq struct {
	StudentID string `json:"student_id"`
	WindowID  string `json:"window_id"`
}

type userReq struct {
	StudentID   string `json:"student_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

func (h *Handler) CreateCanteen(w http.ResponseWriter, r *http.Request) {
	var req nameReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "食堂名称必填")
		return
	}
	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO canteens (name, sort) VALUES ($1, $2) RETURNING id
	`, strings.TrimSpace(req.Name), req.Sort).Scan(&id)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"id": id.String(), "name": strings.TrimSpace(req.Name)})
}

func (h *Handler) CreateFloor(w http.ResponseWriter, r *http.Request) {
	canteenID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "食堂 ID 不正确")
		return
	}
	var req nameReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "楼层名称必填")
		return
	}
	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO floors (canteen_id, name, sort) VALUES ($1, $2, $3) RETURNING id
	`, canteenID, strings.TrimSpace(req.Name), req.Sort).Scan(&id)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"id": id.String(), "canteen_id": canteenID.String()})
}

func (h *Handler) CreateWindow(w http.ResponseWriter, r *http.Request) {
	var req windowReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "窗口名称和编号必填")
		return
	}
	canteenID, err := uuid.Parse(req.CanteenID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "食堂 ID 不正确")
		return
	}
	var floor any
	if strings.TrimSpace(req.FloorID) != "" {
		id, err := uuid.Parse(req.FloorID)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "楼层 ID 不正确")
			return
		}
		floor = id
	}
	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO windows (canteen_id, floor_id, name, code, blurb, sort, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'open')
		RETURNING id
	`, canteenID, floor, strings.TrimSpace(req.Name), strings.TrimSpace(req.Code), strings.TrimSpace(req.Blurb), req.Sort).Scan(&id)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"id": id.String(), "status": "open"})
}

func (h *Handler) Grant(w http.ResponseWriter, r *http.Request) {
	var req grantReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "请求不正确")
		return
	}
	windowID, err := uuid.Parse(req.WindowID)
	if err != nil || strings.TrimSpace(req.StudentID) == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "学号和窗口必填")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		INSERT INTO staff_window_grants (user_id, window_id)
		SELECT id, $2 FROM users WHERE student_id = $1 AND role = 'staff'
		ON CONFLICT DO NOTHING
	`, strings.TrimSpace(req.StudentID), windowID)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = h.pool.QueryRow(r.Context(), `
			SELECT EXISTS(
				SELECT 1 FROM staff_window_grants g
				JOIN users u ON u.id = g.user_id
				WHERE u.student_id = $1 AND g.window_id = $2
			)
		`, strings.TrimSpace(req.StudentID), windowID).Scan(&exists)
		if !exists {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "找不到该员工")
			return
		}
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"granted": true})
}

func (h *Handler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []userReq `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil || len(req.Items) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "items 不能为空")
		return
	}
	if len(req.Items) > 200 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "一次最多导入 200 人")
		return
	}
	created := 0
	for _, item := range req.Items {
		role := item.Role
		if role == "" {
			role = identity.RoleDiner
		}
		if role != identity.RoleDiner && role != identity.RoleStaff && role != identity.RoleAdmin {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "角色不正确")
			return
		}
		if strings.TrimSpace(item.StudentID) == "" || item.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "学号和密码必填")
			return
		}
		hash, err := identity.HashPassword(item.Password)
		if err != nil {
			h.writeDB(w, err)
			return
		}
		tag, err := h.pool.Exec(r.Context(), `
			INSERT INTO users (student_id, password_hash, display_name, role)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (student_id) DO NOTHING
		`, strings.TrimSpace(item.StudentID), hash, strings.TrimSpace(item.DisplayName), role)
		if err != nil {
			h.writeDB(w, err)
			return
		}
		created += int(tag.RowsAffected())
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"created": created, "received": len(req.Items)})
}

func (h *Handler) writeDB(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "编号已存在")
		return
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "关联资源不存在")
		return
	}
	h.log.Error("admin.write", map[string]any{"outcome": "error", "error": err.Error()})
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
}
