package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"jiaohao/internal/config"
)

func main() {
	n := flag.Int("n", 5, "WebSocket 连接数，默认 5，未授权时最多 100")
	hold := flag.Duration("hold", 2*time.Second, "连接保持时间")
	flag.Parse()
	if *n < 1 {
		fmt.Println("n 至少为 1")
		os.Exit(1)
	}
	if *n > 100 && os.Getenv("ALLOW_STRESS") != "1" {
		fmt.Println("超过 100 连接需要单独授权。2000–3000 的压测不要用本默认命令。")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config_fail")
		os.Exit(1)
	}
	diner := login(cfg.SeedDinerStudentID, cfg.SeedDinerPassword)
	windowID := firstWindow(diner)
	ok := 0
	conns := make([]*websocket.Conn, 0, *n)
	for i := 0; i < *n; i++ {
		u := url.URL{Scheme: "ws", Host: "127.0.0.1:8080", Path: "/api/v1/ws", RawQuery: "token=" + url.QueryEscape(diner)}
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			break
		}
		if err := conn.WriteJSON(map[string]any{"type": "subscribe", "topics": []string{"window:" + windowID}}); err != nil {
			_ = conn.Close()
			break
		}
		conns = append(conns, conn)
		ok++
	}
	time.Sleep(*hold)
	for _, conn := range conns {
		_ = conn.Close()
	}
	fmt.Printf("connected=%d requested=%d\n", ok, *n)
	if ok != *n {
		os.Exit(1)
	}
}

func login(studentID, password string) string {
	body, _ := json.Marshal(map[string]string{"student_id": studentID, "password": password})
	res, err := http.Post("http://127.0.0.1:8080/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("login_fail")
		os.Exit(1)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &env) != nil || !env.OK || env.Data.Token == "" {
		fmt.Println("login_fail")
		os.Exit(1)
	}
	return env.Data.Token
}

func firstWindow(token string) string {
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/canteens", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("canteen_fail")
		os.Exit(1)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var env struct {
		Data struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &env)
	var canteenID string
	for _, item := range env.Data.Items {
		if item.Name == "第一食堂" {
			canteenID = item.ID
		}
	}
	req, _ = http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/windows?canteen_id="+canteenID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("window_fail")
		os.Exit(1)
	}
	defer res.Body.Close()
	raw, _ = io.ReadAll(res.Body)
	var windows struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &windows)
	if len(windows.Data.Items) == 0 {
		fmt.Println("window_fail")
		os.Exit(1)
	}
	return windows.Data.Items[0].ID
}
