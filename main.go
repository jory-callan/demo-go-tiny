package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// 统一 JSON 返回
type resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------- 1. 探活 ----------
func ping(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: "pong"})
}

// ---------- 2. 回显 ----------
func echo(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Method == http.MethodPost {
		body, _ = io.ReadAll(r.Body)
	}
	writeJSON(w, http.StatusOK, resp{
		Code: 0,
		Data: map[string]interface{}{
			"method":  r.Method,
			"query":   r.URL.Query(),
			"body":    string(body),
			"headers": r.Header,
		},
	})
}

// ---------- 3. 客户端 IP ----------
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func ip(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, resp{Code: 0, Data: clientIP(r)})
}

// ---------- 4. 环境变量（方便确认 Pod 调度到哪个节点） ----------
func env(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, resp{Code: 0, Data: map[string]string{
		"POD_NAME":   os.Getenv("POD_NAME"),
		"NODE_NAME":  os.Getenv("NODE_NAME"),
		"VERSION":    os.Getenv("VERSION"),
		"START_TIME": startTime.Format(time.RFC3339),
	}})
}

// ---------- 5. 性能：模拟延迟 ----------
func delay(w http.ResponseWriter, r *http.Request) {
	ms := r.URL.Query().Get("ms")
	if ms == "" {
		ms = "100"
	}
	d, _ := time.ParseDuration(ms + "ms")
	time.Sleep(d)
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: "slept " + ms + "ms"})
}

// ---------- 6. 性能：模拟内存分配 ----------
func mem(w http.ResponseWriter, r *http.Request) {
	size := 1024 * 1024 // 默认 1 MiB
	if n := r.URL.Query().Get("mb"); n != "" {
		fmt.Sscanf(n, "%d", &size)
		size *= 1024 * 1024
	}
	_ = make([]byte, size) // 简单占住
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("allocated %d MiB", size/1024/1024)})
}

var startTime = time.Now()

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/echo", echo)
	mux.HandleFunc("/ip", ip)
	mux.HandleFunc("/env", env)
	mux.HandleFunc("/delay", delay)
	mux.HandleFunc("/mem", mem)

	// 7. 根路径提示
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
