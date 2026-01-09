package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
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
	// 解析参数
	sizeMB := 1 // 默认 1 MiB
	if n := r.URL.Query().Get("mb"); n != "" {
		fmt.Sscanf(n, "%d", &sizeMB)
	}
	size := sizeMB * 1024 * 1024

	// 解析保持时长参数（ms），默认0表示立即释放
	durationMs := 0
	if d := r.URL.Query().Get("duration"); d != "" {
		fmt.Sscanf(d, "%d", &durationMs)
	}

	// 如果没有指定保持时长，直接分配内存后返回
	if durationMs == 0 {
		_ = make([]byte, size)
		writeJSON(w, http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("allocated %d MiB", sizeMB)})
		return
	}

	// 转换为时间类型
	duration := time.Duration(durationMs) * time.Millisecond
	// 计算提升阶段和保持阶段的时长
	increaseDuration := duration / 2
	maintainDuration := duration - increaseDuration

	// 创建一个slice来存储分配的内存块，以便在goroutine结束后释放
	var memBlocks [][]byte
	memBlocks = append(memBlocks, make([]byte, 0)) // 初始化一个空slice

	// 启动goroutine来处理内存分配
	go func() {
		defer func() {
			// 释放所有内存
			memBlocks = nil
			runtime.GC() // 触发垃圾回收
		}()

		// 提升阶段：缓步增加内存占用
		stepCount := 100 // 分100步提升内存
		stepSize := size / stepCount
		stepDuration := increaseDuration / time.Duration(stepCount)

		for i := 1; i <= stepCount; i++ {
			currentSize := i * stepSize
			memBlocks = append(memBlocks, make([]byte, currentSize))
			time.Sleep(stepDuration)
		}

		// 保持阶段：保持内存占用
		time.Sleep(maintainDuration)
	}()

	// 返回响应
	writeJSON(w, http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("allocating %d MiB over %d ms, maintaining for %d ms", sizeMB, increaseDuration.Milliseconds(), maintainDuration.Milliseconds())})
}

// ---------- 7. 性能：模拟CPU占用 ----------
func cpu(w http.ResponseWriter, r *http.Request) {
	// 参数解析
	durationMs := r.URL.Query().Get("ms")
	if durationMs == "" {
		durationMs = "1000" // 默认1秒
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

func main() {
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
			"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10&duration=10000 /cpu?ms=1000&cores=2&percent=80",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
