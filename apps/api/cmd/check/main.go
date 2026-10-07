package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"jiaohao/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail("config", err.Error())
	}
	c := &client{base: "http://127.0.0.1:8080"}
	diner := c.login(cfg.SeedDinerStudentID, cfg.SeedDinerPassword)
	staff := c.login(cfg.SeedStaffStudentID, cfg.SeedStaffPassword)
	admin := c.login(cfg.AdminStudentID, cfg.AdminPassword)

	c.want("anon-canteens", http.MethodGet, "/api/v1/canteens", "", nil, 401, "UNAUTHENTICATED")

	canteens := c.get("/api/v1/canteens", diner)
	var c1, c2 string
	for _, item := range items(canteens) {
		switch item["name"] {
		case "第一食堂":
			c1 = str(item["id"])
		case "第二食堂":
			c2 = str(item["id"])
		}
	}
	if c1 == "" || c2 == "" {
		fail("browse", "缺少两个演示食堂")
	}
	pass("browse-two-canteens")

	var w1, w2 string
	for _, item := range items(c.get("/api/v1/windows?canteen_id="+c1, diner)) {
		switch item["code"] {
		case "W1":
			w1 = str(item["id"])
		case "W2":
			w2 = str(item["id"])
		}
	}
	if w1 == "" || w2 == "" {
		fail("browse", "第一食堂缺少 W1/W2")
	}
	pass("browse-windows")
	clearActive(c, diner, staff, w1)

	first := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	second := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	if str(data(first)["id"]) == "" || str(data(first)["id"]) != str(data(second)["id"]) {
		fail("idempotent-take", "重复取号没有返回同一张票")
	}
	pass("idempotent-take")
	c.want("cross-window", http.MethodPost, "/api/v1/tickets", diner, map[string]string{"window_id": w2}, 409, "HAS_ACTIVE_TICKET")
	c.want("diner-call", http.MethodPost, "/api/v1/windows/"+w1+"/call-next", diner, nil, 403, "FORBIDDEN")
	c.post("/api/v1/tickets/"+str(data(first)["id"])+"/cancel", diner, nil)
	if c.get("/api/v1/me/ticket", diner)["data"] != nil {
		fail("cancel", "取消后仍有有效号")
	}
	pass("cancel")

	codes := map[string]bool{}
	for _, item := range items(c.get("/api/v1/staff/windows", staff)) {
		codes[str(item["code"])] = true
	}
	if !codes["W1"] || codes["W2"] {
		fail("staff-scope", "员工窗口范围不正确")
	}
	pass("staff-scope")
	c.want("staff-ungranted", http.MethodPost, "/api/v1/windows/"+w2+"/call-next", staff, nil, 403, "FORBIDDEN")

	taken := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	called := c.post("/api/v1/windows/"+w1+"/call-next", staff, nil)
	if str(data(called)["status"]) != "called" {
		fail("call-next", "状态不是 called")
	}
	pass("call-next")
	c.want("double-call", http.MethodPost, "/api/v1/windows/"+w1+"/call-next", staff, nil, 409, "CALLED_PENDING")
	c.want("cancel-called", http.MethodPost, "/api/v1/tickets/"+str(data(taken)["id"])+"/cancel", diner, nil, 409, "TICKET_NOT_WAITING")
	if str(data(c.post("/api/v1/windows/"+w1+"/complete", staff, nil))["status"]) != "completed" {
		fail("complete", "状态不是 completed")
	}
	pass("complete")

	c.want("display-anon", http.MethodGet, "/api/v1/display/windows/"+w1, "", nil, 401, "DISPLAY_TOKEN_INVALID")

	created := c.post("/api/v1/admin/canteens", admin, map[string]any{"name": "回归食堂", "sort": 80})
	cid := str(data(created)["id"])
	renamed := c.send(http.MethodPatch, "/api/v1/admin/canteens/"+cid, admin, map[string]any{"name": "回归食堂已改名", "sort": 80})
	if str(data(renamed)["name"]) != "回归食堂已改名" {
		fail("rename", "改名未生效")
	}
	pass("rename")
	win := c.post("/api/v1/admin/windows", admin, map[string]any{"canteen_id": cid, "name": "回归窗口", "code": "QA9", "sort": 1})
	c.send(http.MethodDelete, "/api/v1/admin/windows/"+str(data(win)["id"]), admin, nil)
	c.send(http.MethodDelete, "/api/v1/admin/canteens/"+cid, admin, nil)
	pass("delete-empty")
	c.want("delete-used-window", http.MethodDelete, "/api/v1/admin/windows/"+w1, admin, nil, 409, "CONFLICT")

	studentID := "9" + time.Now().Format("0102150405")
	body := "student_id,display_name,role,password\n" + studentID + ",回归导入,diner,check-pass-1\n"
	imported := c.sendRaw(http.MethodPost, "/api/v1/admin/users/import-file", admin, "text/csv", []byte(body))
	if intNum(data(imported)["created"]) != 1 {
		fail("csv-import", "没有新建账号")
	}
	again := c.sendRaw(http.MethodPost, "/api/v1/admin/users/import-file", admin, "text/csv", []byte(body))
	if intNum(data(again)["created"]) != 0 {
		fail("csv-import", "重复导入不应新建")
	}
	c.login(studentID, "check-pass-1")
	pass("csv-import")

	c.post("/api/v1/windows/"+w1+"/pause-take", staff, nil)
	c.want("pause-take", http.MethodPost, "/api/v1/tickets", diner, map[string]string{"window_id": w1}, 409, "WINDOW_PAUSE_TAKE")
	c.post("/api/v1/windows/"+w1+"/resume-take", staff, nil)
	pass("pause-resume")

	accept(c, diner, staff, admin, c1, c2, w1, studentID)
	fmt.Println("check ok")
}

func accept(c *client, diner, staff, admin, c1, c2, w1, secondID string) {
	other := ""
	var otherBefore map[string]any
	for _, item := range items(c.get("/api/v1/windows?canteen_id="+c2, diner)) {
		other = str(item["id"])
		otherBefore = item
		break
	}
	if other == "" {
		fail("isolation", "第二食堂没有窗口")
	}

	second := c.login(secondID, "check-pass-1")
	releaseOwn(c, diner, staff, w1)
	releaseOwn(c, second, staff, w1)
	w := windowByID(c, diner, c1, w1)
	if intNum(w["waiting_count"]) != 0 || intNum(w["current_number"]) != 0 {
		fail("quiet-window", "W1 上有其他有效号，停止以免误叫")
	}
	pass("quiet-window")

	taken := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	if intNum(data(taken)["people_ahead"]) != 0 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("people-ahead", "第一位等待的人前面不应有人")
	}
	if intNum(windowByID(c, diner, c1, w1)["waiting_count"]) != 1 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("waiting-count", "取号后等待人数不是 1")
	}
	behind := c.post("/api/v1/tickets", second, map[string]string{"window_id": w1})
	if intNum(data(behind)["people_ahead"]) != 1 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("people-ahead", "第二位前面应有 1 人")
	}
	if intNum(windowByID(c, diner, c1, w1)["waiting_count"]) != 2 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("waiting-count", "两人等待时人数不是 2")
	}
	c.post("/api/v1/tickets/"+str(data(behind)["id"])+"/cancel", second, nil)
	if intNum(windowByID(c, diner, c1, w1)["waiting_count"]) != 1 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("waiting-count", "取消后等待人数没有减 1")
	}
	pass("waiting-count")
	c.post("/api/v1/tickets/"+str(data(taken)["id"])+"/cancel", diner, nil)

	skipped := takeThenSkip(c, diner, staff, w1)
	if str(data(skipped)["status"]) != "skipped" {
		restoreDemo(c, staff, admin, diner, w1)
		fail("manual-skip", "状态不是 skipped")
	}
	if c.get("/api/v1/me/ticket", diner)["data"] != nil {
		restoreDemo(c, staff, admin, diner, w1)
		fail("manual-skip", "过号后仍有有效号")
	}
	again := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	if intNum(data(again)["number"]) != intNum(data(skipped)["number"])+1 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("manual-skip", "过号后的新号没有递增")
	}
	c.post("/api/v1/tickets/"+str(data(again)["id"])+"/cancel", diner, nil)
	pass("manual-skip")

	held := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	c.post("/api/v1/windows/"+w1+"/close", staff, nil)
	closed := windowByID(c, diner, c1, w1)
	if str(closed["status"]) != "closed" || str(closed["id"]) == "" {
		c.post("/api/v1/windows/"+w1+"/open", staff, nil)
		restoreDemo(c, staff, admin, diner, w1)
		fail("closed-visible", "打烊后列表里看不到该窗口")
	}
	c.post("/api/v1/tickets/"+str(data(held)["id"])+"/cancel", diner, nil)
	denied := c.send(http.MethodPost, "/api/v1/tickets", diner, map[string]string{"window_id": w1})
	if intNum(denied["_status"]) != 409 || errCode(denied) != "WINDOW_NOT_OPEN" {
		c.post("/api/v1/windows/"+w1+"/open", staff, nil)
		fail("closed-no-take", fmt.Sprintf("status=%v code=%s", denied["_status"], errCode(denied)))
	}
	c.post("/api/v1/windows/"+w1+"/open", staff, nil)
	queued := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	c.post("/api/v1/windows/"+w1+"/close", staff, nil)
	called := c.post("/api/v1/windows/"+w1+"/call-next", staff, nil)
	c.post("/api/v1/windows/"+w1+"/open", staff, nil)
	if str(data(called)["id"]) != str(data(queued)["id"]) || str(data(called)["status"]) != "called" {
		restoreDemo(c, staff, admin, diner, w1)
		fail("closed-call", "打烊后没有叫到当日等待号")
	}
	c.post("/api/v1/windows/"+w1+"/complete", staff, nil)
	pass("closed-call")

	issued := c.post("/api/v1/windows/"+w1+"/display-token", staff, nil)
	token := str(data(issued)["token"])
	if token == "" {
		restoreDemo(c, staff, admin, diner, w1)
		fail("display-token", "没有签发令牌")
	}
	view := c.get("/api/v1/display/windows/"+w1+"?token="+url.QueryEscape(token), "")
	if intNum(view["_status"]) != 200 || str(data(view)["window_id"]) != w1 {
		restoreDemo(c, staff, admin, diner, w1)
		fail("display-token", "有效令牌读不到大屏")
	}
	wrong := c.get("/api/v1/display/windows/"+w1+"?token=not-this-token", "")
	c.send(http.MethodDelete, "/api/v1/windows/"+w1+"/display-token", staff, nil)
	if intNum(wrong["_status"]) != 401 || errCode(wrong) != "DISPLAY_TOKEN_INVALID" {
		fail("display-wrong", fmt.Sprintf("status=%v code=%s", wrong["_status"], errCode(wrong)))
	}
	pass("display-wrong")
	revoked := c.get("/api/v1/display/windows/"+w1+"?token="+url.QueryEscape(token), "")
	if intNum(revoked["_status"]) != 401 || errCode(revoked) != "DISPLAY_TOKEN_INVALID" {
		fail("display-revoke", fmt.Sprintf("status=%v code=%s", revoked["_status"], errCode(revoked)))
	}
	pass("display-revoke")

	setTimeout(c, admin, w1, 2)
	auto := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	c.post("/api/v1/windows/"+w1+"/call-next", staff, nil)
	setTimeout(c, admin, w1, 0)
	if !waitNoTicket(c, diner, 8*time.Second) {
		restoreDemo(c, staff, admin, diner, w1)
		fail("auto-skip", "2 秒过号配置到期后仍有有效号")
	}
	if intNum(data(auto)["number"]) == 0 {
		fail("auto-skip", "没有取到号")
	}
	pass("auto-skip")

	heldZero := c.post("/api/v1/tickets", diner, map[string]string{"window_id": w1})
	c.post("/api/v1/windows/"+w1+"/call-next", staff, nil)
	time.Sleep(3 * time.Second)
	mine := c.get("/api/v1/me/ticket", diner)
	if str(data(mine)["status"]) != "called" || str(data(mine)["id"]) != str(data(heldZero)["id"]) {
		restoreDemo(c, staff, admin, diner, w1)
		fail("no-auto-skip", "0 秒配置不应自动过号")
	}
	c.post("/api/v1/windows/"+w1+"/complete", staff, nil)
	pass("no-auto-skip")

	after := windowByID(c, diner, c2, other)
	if intNum(after["current_number"]) != intNum(otherBefore["current_number"]) || intNum(after["waiting_count"]) != intNum(otherBefore["waiting_count"]) {
		fail("isolation", "第一食堂叫号改变了第二食堂窗口")
	}
	pass("isolation")
}

func windowByID(c *client, token, canteenID, windowID string) map[string]any {
	for _, item := range items(c.get("/api/v1/windows?canteen_id="+canteenID, token)) {
		if str(item["id"]) == windowID {
			return item
		}
	}
	return map[string]any{}
}

func releaseOwn(c *client, diner, staff, windowID string) {
	mine := c.get("/api/v1/me/ticket", diner)
	ticket, _ := mine["data"].(map[string]any)
	if ticket == nil {
		return
	}
	switch str(ticket["status"]) {
	case "waiting":
		c.post("/api/v1/tickets/"+str(ticket["id"])+"/cancel", diner, nil)
	case "called":
		if str(ticket["window_id"]) == windowID {
			c.post("/api/v1/windows/"+windowID+"/complete", staff, nil)
		}
	}
}

func takeThenSkip(c *client, diner, staff, windowID string) map[string]any {
	c.post("/api/v1/tickets", diner, map[string]string{"window_id": windowID})
	c.post("/api/v1/windows/"+windowID+"/call-next", staff, nil)
	return c.post("/api/v1/windows/"+windowID+"/skip", staff, nil)
}

func setTimeout(c *client, admin, windowID string, seconds int) {
	env := c.send(http.MethodPatch, "/api/v1/admin/windows/"+windowID, admin, map[string]any{"skip_timeout_seconds": seconds})
	if intNum(env["_status"]) != 200 {
		fail("skip-timeout", fmt.Sprintf("status=%v", env["_status"]))
	}
}

func restoreDemo(c *client, staff, admin, diner, windowID string) {
	c.post("/api/v1/windows/"+windowID+"/open", staff, nil)
	setTimeout(c, admin, windowID, 0)
	c.send(http.MethodDelete, "/api/v1/windows/"+windowID+"/display-token", staff, nil)
	releaseOwn(c, diner, staff, windowID)
}

func waitNoTicket(c *client, diner string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		if c.get("/api/v1/me/ticket", diner)["data"] == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func clearActive(c *client, diner, staff, windowID string) {
	mine := c.get("/api/v1/me/ticket", diner)
	ticket, _ := mine["data"].(map[string]any)
	if ticket == nil {
		return
	}
	switch str(ticket["status"]) {
	case "waiting":
		c.post("/api/v1/tickets/"+str(ticket["id"])+"/cancel", diner, nil)
	case "called":
		c.post("/api/v1/windows/"+windowID+"/complete", staff, nil)
	}
}

type client struct{ base string }

type envelope struct {
	OK    bool `json:"ok"`
	Data  any  `json:"data"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
	Code int
}

func (c *client) login(studentID, password string) string {
	env := c.post("/api/v1/auth/login", "", map[string]string{"student_id": studentID, "password": password})
	token := str(data(env)["token"])
	if token == "" {
		fail("login", studentID)
	}
	return token
}

func (c *client) get(path, token string) map[string]any {
	return c.send(http.MethodGet, path, token, nil)
}

func (c *client) post(path, token string, body any) map[string]any {
	return c.send(http.MethodPost, path, token, body)
}

func (c *client) send(method, path, token string, body any) map[string]any {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	kind := ""
	if body != nil {
		kind = "application/json"
	}
	return c.sendRaw(method, path, token, kind, raw)
}

func (c *client) sendRaw(method, path, token, contentType string, raw []byte) map[string]any {
	var reader io.Reader
	if raw != nil {
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		fail(path, err.Error())
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(path, err.Error())
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(res.Body)
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		fail(path, "响应不是 JSON")
	}
	env["_status"] = res.StatusCode
	if res.StatusCode >= 400 && !strings.Contains(path, "want-error") {
		// callers that expect errors use want()
	}
	return env
}

func (c *client) want(name, method, path, token string, body any, status int, code string) {
	env := c.send(method, path, token, body)
	if intNum(env["_status"]) != status || errCode(env) != code {
		fail(name, fmt.Sprintf("status=%v code=%s", env["_status"], errCode(env)))
	}
	pass(name)
}

func data(env map[string]any) map[string]any {
	m, _ := env["data"].(map[string]any)
	if m == nil {
		fail("data", "缺少 data")
	}
	return m
}

func items(env map[string]any) []map[string]any {
	raw, _ := data(env)["items"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func errCode(env map[string]any) string {
	m, _ := env["error"].(map[string]any)
	if m == nil {
		return ""
	}
	return str(m["code"])
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func intNum(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}

func pass(name string) { fmt.Println("pass", name) }

func fail(name, detail string) {
	fmt.Printf("fail %s %s\n", name, detail)
	os.Exit(1)
}
