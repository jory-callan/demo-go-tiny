package use_mux

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// 统一 JSON 返回结构
type resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

var startTime = time.Now()

// writeJSON 统一JSON响应函数
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// StartServer 启动Gorilla Mux服务
func StartServer() {
	router := mux.NewRouter()

	// 注册路由
	registerRoutes(router)

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Gorilla Mux server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

// registerRoutes 注册所有路由
func registerRoutes(router *mux.Router) {
	router.HandleFunc("/ping", pingHandler).Methods("GET")
	router.HandleFunc("/echo", echoHandler)
	router.HandleFunc("/ip", ipHandler).Methods("GET")
	router.HandleFunc("/env", envHandler).Methods("GET")
	router.HandleFunc("/delay", delayHandler).Methods("GET")
	router.HandleFunc("/mem", memHandler).Methods("GET")
	router.HandleFunc("/cpu", cpuHandler).Methods("GET")
	router.HandleFunc("/", rootHandler).Methods("GET")
}

// ---------- 1. 探活 ----------
func pingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: "pong"})
}

// ---------- 2. 回显 ----------
func echoHandler(w http.ResponseWriter, r *http.Request) {
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

func ipHandler(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	writeJSON(w, http.StatusOK, resp{Code: 0, Data: ip})
}

// ---------- 4. 环境变量 ----------
func envHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, resp{Code: 0, Data: map[string]string{
		"POD_NAME":   os.Getenv("POD_NAME"),
		"NODE_NAME":  os.Getenv("NODE_NAME"),
		"VERSION":    os.Getenv("VERSION"),
		"START_TIME": startTime.Format(time.RFC3339),
	}})
}

// ---------- 5. 延迟模拟 ----------
func delayHandler(w http.ResponseWriter, r *http.Request) {
	ms := r.URL.Query().Get("ms")
	if ms == "" {
		ms = "100"
	}
	d, _ := time.ParseDuration(ms + "ms")
	time.Sleep(d)
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: "slept " + ms + "ms"})
}

// ---------- 6. 内存分配模拟 ----------
func memHandler(w http.ResponseWriter, r *http.Request) {
	sizeMB := 1
	if v := r.URL.Query().Get("mb"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sizeMB = n
		}
	}
	durationMs := 2000
	if v := r.URL.Query().Get("ms"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			durationMs = n
		}
	}

	size := sizeMB * 1024 * 1024
	buf := make([]byte, size)

	// 触发实际物理内存分配
	const page = 4096
	for i := 0; i < len(buf); i += page {
		buf[i] = 1
	}

	// 保持内存占用
	time.Sleep(time.Duration(durationMs) * time.Millisecond)

	// 释放内存
	buf = nil
	runtime.GC()
	debug.FreeOSMemory()

	writeJSON(w, http.StatusOK, resp{
		Code: 0,
		Msg:  fmt.Sprintf("allocated %d MiB for %d ms", sizeMB, durationMs),
	})
}

// ---------- 7. CPU占用模拟 ----------
func cpuHandler(w http.ResponseWriter, r *http.Request) {
	// 参数解析
	durationMs := r.URL.Query().Get("ms")
	if durationMs == "" {
		durationMs = "2000"
	}
	duration, _ := time.ParseDuration(durationMs + "ms")

	// CPU核心数参数
	coresStr := r.URL.Query().Get("cores")
	cores := runtime.NumCPU()
	if coresStr != "" {
		if c, err := strconv.Atoi(coresStr); err == nil && c > 0 {
			cores = c
			if cores > runtime.NumCPU() {
				cores = runtime.NumCPU()
			}
		}
	}

	// CPU使用率参数
	percentStr := r.URL.Query().Get("percent")
	percent := 80
	if percentStr != "" {
		if p, err := strconv.Atoi(percentStr); err == nil && p > 0 && p <= 100 {
			percent = p
		}
	}

	var wg sync.WaitGroup

	// 启动goroutine占用CPU
	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cpuIntensiveTask(duration, percent)
		}()
	}

	// 异步等待完成
	go func() {
		wg.Wait()
		writeJSON(w, http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("CPU test completed: %d core(s) at %d%% for %s", cores, percent, duration)})
	}()

	w.WriteHeader(http.StatusAccepted)
}

// cpuIntensiveTask 执行CPU密集型任务
func cpuIntensiveTask(duration time.Duration, percent int) {
	endTime := time.Now().Add(duration)

	for time.Now().Before(endTime) {
		// 工作周期
		workStart := time.Now()
		workDuration := time.Duration(float64(time.Millisecond*10) * float64(percent) / 100)

		// 执行CPU密集型计算
		counter := 0
		for time.Since(workStart) < workDuration {
			counter++
			_ = rand.Float64() * rand.Float64()
		}

		// 休息周期
		restDuration := time.Millisecond*10 - workDuration
		if restDuration > 0 {
			time.Sleep(restDuration)
		}
	}
}

// ---------- 8. 根路径提示 ----------
func rootHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10&ms=10000 /cpu?ms=1000&cores=2&percent=80",
	})
}
