package use_gin

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

	"github.com/gin-gonic/gin"
)

// 统一 JSON 返回
type resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// 启动Gin服务
func StartServer() {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin引擎
	r := gin.Default()

	// ---------- 1. 探活 ----------
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, resp{Code: 0, Msg: "pong"})
	})

	// ---------- 2. 回显 ----------
	r.Any("/echo", func(c *gin.Context) {
		var body []byte
		if c.Request.Method == http.MethodPost {
			body, _ = io.ReadAll(c.Request.Body)
		}
		c.JSON(http.StatusOK, resp{
			Code: 0,
			Data: map[string]interface{}{
				"method":  c.Request.Method,
				"query":   c.Request.URL.Query(),
				"body":    string(body),
				"headers": c.Request.Header,
			},
		})
	})

	// ---------- 3. 客户端 IP ----------
	r.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, resp{Code: 0, Data: clientIP(c.Request)})
	})

	// ---------- 4. 环境变量 ----------
	r.GET("/env", func(c *gin.Context) {
		c.JSON(http.StatusOK, resp{Code: 0, Data: map[string]string{
			"POD_NAME":   os.Getenv("POD_NAME"),
			"NODE_NAME":  os.Getenv("NODE_NAME"),
			"VERSION":    os.Getenv("VERSION"),
			"START_TIME": startTime.Format(time.RFC3339),
		}})
	})

	// ---------- 5. 性能：模拟延迟 ----------
	r.GET("/delay", func(c *gin.Context) {
		ms := c.Query("ms")
		if ms == "" {
			ms = "100"
		}
		d, _ := time.ParseDuration(ms + "ms")
		time.Sleep(d)
		c.JSON(http.StatusOK, resp{Code: 0, Msg: "slept " + ms + "ms"})
	})

	// ---------- 6. 性能：模拟内存分配 ----------
	r.GET("/mem", func(c *gin.Context) {
		sizeMB := 1
		if v := c.Query("mb"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				sizeMB = n
			}
		}
		durationMs := 2000
		if v := c.Query("ms"); v != "" {
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

		// 保持 buf 引用，sleep 期间不释放
		time.Sleep(time.Duration(durationMs) * time.Millisecond)

		// 手动触发 GC 以确保内存及时释放
		buf = nil
		runtime.GC()
		debug.FreeOSMemory()

		// 返回响应
		c.JSON(http.StatusOK, resp{
			Code: 0,
			Msg:  fmt.Sprintf("allocated %d MiB for %d ms", sizeMB, durationMs),
		})
	})

	// ---------- 7. 性能：模拟CPU占用 ----------
	r.GET("/cpu", func(c *gin.Context) {
		// 参数解析
		durationMs := c.Query("ms")
		if durationMs == "" {
			durationMs = "2000" // 默认2秒
		}
		duration, _ := time.ParseDuration(durationMs + "ms")

		// CPU核心数参数
		coresStr := c.Query("cores")
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
		percentStr := c.Query("percent")
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

		// 异步等待并返回结果
		go func() {
			wg.Wait()
			c.JSON(http.StatusOK, resp{Code: 0, Msg: fmt.Sprintf("CPU test completed: %d core(s) at %d%% for %s", cores, percent, duration)})
		}()
	})

	// ---------- 8. 根路径提示 ----------
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"routes": "/ping /echo /ip /env /delay?ms=100 /mem?mb=10&ms=10000 /cpu?ms=1000&cores=2&percent=80",
		})
	})

	// 监听端口
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Gin server listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// 客户端IP获取函数
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

var startTime = time.Now()
