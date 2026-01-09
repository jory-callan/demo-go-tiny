package use_echo

import (
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

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// 统一 JSON 返回结构
type resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

var startTime = time.Now()

// StartServer 启动Echo服务
func StartServer() {
	e := echo.New()

	// 中间件
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 注册路由
	registerRoutes(e)

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Echo server listening on :%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}

// registerRoutes 注册所有路由
func registerRoutes(e *echo.Echo) {
	e.GET("/ping", pingHandler)
	e.Any("/echo", echoHandler)
	e.GET("/ip", ipHandler)
	e.GET("/env", envHandler)
	e.GET("/delay", delayHandler)
	e.GET("/mem", memHandler)
	e.GET("/cpu", cpuHandler)
	e.GET("/", rootHandler)
}

// ---------- 1. 探活 ----------
func pingHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, resp{Code: 0, Msg: "pong"})
}

// ---------- 2. 回显 ----------
func echoHandler(c echo.Context) error {
	var body []byte
	if c.Request().Method == http.MethodPost {
		body, _ = io.ReadAll(c.Request().Body)
	}
	return c.JSON(http.StatusOK, resp{
		Code: 0,
		Data: map[string]interface{}{
			"method":  c.Request().Method,
			"query":   c.Request().URL.Query(),
			"body":    string(body),
			"headers": c.Request().Header,
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

func ipHandler(c echo.Context) error {
	ip := clientIP(c.Request())
	return c.JSON(http.StatusOK, resp{Code: 0, Data: ip})
}

// ---------- 4. 环境变量 ----------
func envHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, resp{Code: 0, Data: map[string]string{
		"POD_NAME":   os.Getenv("POD_NAME"),
		"NODE_NAME":  os.Getenv("NODE_NAME"),
		"VERSION":    os.Getenv("VERSION"),
		"START_TIME": startTime.Format(time.RFC3339),
	}})
}

// ---------- 5. 延迟模拟 ----------
func delayHandler(c echo.Context) error {
	ms := c.QueryParam("ms")
	if ms == "" {
		ms = "100"
	}
	d, _ := time.ParseDuration(ms + "ms")
	time.Sleep(d)
	return c.JSON(http.StatusOK, resp{Code: 0, Msg: "slept " + ms + "ms"})
}

// ---------- 6. 内存分配模拟 ----------
func memHandler(c echo.Context) error {
	sizeMB := 1
	if v := c.QueryParam("mb"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sizeMB = n
		}
	}
	durationMs := 2000
	if v := c.QueryParam("ms"); v != "" {
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

	return c.JSON(http.StatusOK, resp{
		Code: 0,
		Msg:  fmt.Sprintf("allocated %d MiB for %d ms", sizeMB, durationMs),
	})
}

// ---------- 7. CPU占用模拟 ----------
func cpuHandler(c echo.Context) error {
	// 参数解析
	durationMs := c.QueryParam("ms")
	if durationMs == "" {
		durationMs = "2000"
	}
	duration, _ := time.ParseDuration(durationMs + "ms")

	// CPU核心数参数
	coresStr := c.QueryParam("cores")
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
	percentStr := c.QueryParam("percent")
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
		c.JSON(http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("CPU test completed: %d core(s) at %d%% for %s", cores, percent, duration)})
	}()

	return c.NoContent(http.StatusAccepted)
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
func rootHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10&ms=10000 /cpu?ms=1000&cores=2&percent=80",
	})
}