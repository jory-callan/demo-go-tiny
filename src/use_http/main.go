package use_http

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

	// 触发实际物理内存分配（可选，但推荐）
	const page = 4096
	for i := 0; i < len(buf); i += page {
		buf[i] = 1 // 非零值更可靠触发分配
	}

	// 保持 buf 引用，sleep 期间不释放
	time.Sleep(time.Duration(durationMs) * time.Millisecond)

	// 手动触发 GC 以确保内存及时释放
	buf = nil
	runtime.GC()
	debug.FreeOSMemory() // 强制归还空闲内存给 OS, 禁止在生产环境使用

	// 此时才释放（函数返回自动释放，无需显式 GC）
	writeJSON(w, http.StatusOK, resp{
		Code: 0,
		Msg:  fmt.Sprintf("allocated %d MiB for %d ms", sizeMB, durationMs),
	})
}

// ---------- 7. 性能：模拟CPU占用 ----------
func cpu(w http.ResponseWriter, r *http.Request) {
	// 参数解析
	durationMs := r.URL.Query().Get("ms")
	if durationMs == "" {
		durationMs = "2000" // 默认2秒
	}
	duration, _ := time.ParseDuration(durationMs + "ms")

	// CPU核心数参数
	coresStr := r.URL.Query().Get("cores")
	cores := runtime.NumCPU() // 默认使用所有核心
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
	percent := 80 // 默认80%
	if percentStr != "" {
		if p, err := strconv.Atoi(percentStr); err == nil && p > 0 && p <= 100 {
			percent = p
		}
	}

	var wg sync.WaitGroup

	// 启动指定数量的goroutine来占用CPU
	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 执行CPU密集型任务，控制占用率
			endTime := time.Now().Add(duration)

			for time.Now().Before(endTime) {
				// 工作周期
				workStart := time.Now()
				workDuration := time.Duration(float64(time.Millisecond*10) * float64(percent) / 100)

				// 执行CPU密集型计算
				counter := 0
				for time.Since(workStart) < workDuration {
					counter++
					// 执行一些计算操作
					_ = rand.Float64() * rand.Float64()
				}

				// 休息周期（不占用CPU）
				restDuration := time.Millisecond*10 - workDuration
				if restDuration > 0 {
					time.Sleep(restDuration)
				}
			}
		}()
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		writeJSON(w, http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("CPU test completed: %d core(s) at %d%% for %s", cores, percent, duration)})
	}()
}

var startTime = time.Now()

func StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/echo", echo)
	mux.HandleFunc("/ip", ip)
	mux.HandleFunc("/env", env)
	mux.HandleFunc("/delay", delay)
	mux.HandleFunc("/mem", mem)
	mux.HandleFunc("/cpu", cpu) // 添加CPU占用路由

	// 7. 根路径提示
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10&ms=10000 /cpu?ms=1000&cores=2&percent=80",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
