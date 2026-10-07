package admin

import (
	"encoding/csv"
	"errors"
	"io"
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

func (h *Handler) RenameCanteen(w http.ResponseWriter, r *http.Request) {
	h.rename(w, r, "canteens")
}

func (h *Handler) RenameFloor(w http.ResponseWriter, r *http.Request) {
	h.rename(w, r, "floors")
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request, table string) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "ID 不正确")
		return
	}
	var req nameReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "名称必填")
		return
	}
	var query string
	switch table {
	case "canteens":
		query = `UPDATE canteens SET name = $2, sort = $3, updated_at = now() WHERE id = $1`
	case "floors":
		query = `UPDATE floors SET name = $2, sort = $3, updated_at = now() WHERE id = $1`
	default:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
		return
	}
	tag, err := h.pool.Exec(r.Context(), query, id, strings.TrimSpace(req.Name), req.Sort)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "资源不存在")
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"id": id.String(), "name": strings.TrimSpace(req.Name)})
}

func (h *Handler) DeleteCanteen(w http.ResponseWriter, r *http.Request) {
	h.deleteByID(w, r, `DELETE FROM canteens WHERE id = $1`)
}

func (h *Handler) DeleteFloor(w http.ResponseWriter, r *http.Request) {
	h.deleteByID(w, r, `DELETE FROM floors WHERE id = $1`)
}

func (h *Handler) DeleteWindow(w http.ResponseWriter, r *http.Request) {
	h.deleteByID(w, r, `DELETE FROM windows WHERE id = $1`)
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request, query string) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "ID 不正确")
		return
	}
	tag, err := h.pool.Exec(r.Context(), query, id)
	if err != nil {
		h.writeDB(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "资源不存在")
		return
	}
	httpx.WriteOK(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []userReq `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil || len(req.Items) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "items 不能为空")
		return
	}
	h.importItems(w, r, req.Items)
}

func (h *Handler) ImportUsersFile(w http.ResponseWriter, r *http.Request) {
	var reader io.Reader = r.Body
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "文件无法解析")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "请上传 file 字段")
			return
		}
		defer file.Close()
		reader = file
	}
	rows, err := csv.NewReader(reader).ReadAll()
	if err != nil || len(rows) < 2 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "CSV 至少要有表头和一行")
		return
	}
	head := map[string]int{}
	for i, col := range rows[0] {
		head[strings.TrimSpace(strings.TrimPrefix(col, "\ufeff"))] = i
	}
	for _, key := range []string{"student_id", "password"} {
		if _, ok := head[key]; !ok {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "CSV 需要 student_id,password 列")
			return
		}
	}
	items := make([]userReq, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(strings.TrimSpace(strings.Join(row, ""))) == 0 {
			continue
		}
		item := userReq{StudentID: cell(row, head, "student_id"), Password: cell(row, head, "password")}
		item.DisplayName = cell(row, head, "display_name")
		item.Role = cell(row, head, "role")
		items = append(items, item)
	}
	h.importItems(w, r, items)
}

func cell(row []string, head map[string]int, key string) string {
	i, ok := head[key]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func (h *Handler) importItems(w http.ResponseWriter, r *http.Request, items []userReq) {
	if len(items) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "没有可导入的行")
		return
	}
	if len(items) > 200 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidationError, "一次最多导入 200 人")
		return
	}
	created := 0
	for _, item := range items {
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
	httpx.WriteOK(w, http.StatusOK, map[string]any{"created": created, "received": len(items)})
}

func (h *Handler) writeDB(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "编号已存在")
		return
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if pgErr.ConstraintName == "tickets_window_id_fkey" || pgErr.ConstraintName == "windows_canteen_id_fkey" {
			httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "仍有叫号记录，不能删除")
			return
		}
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "关联资源不存在")
		return
	}
	h.log.Error("admin.write", map[string]any{"outcome": "error", "error": err.Error()})
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务内部错误")
}
