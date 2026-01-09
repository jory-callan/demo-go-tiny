package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"demo-go-tiny/src/use_echo"
	"demo-go-tiny/src/use_gin"
	"demo-go-tiny/src/use_http"
	"demo-go-tiny/src/use_mux"
)

func main() {
	// 解析命令行参数
	var framework string
	flag.StringVar(&framework, "c", "gin", "Specify web framework: gin, echo, mux, http")
	flag.Parse()

	// 启动对应的框架服务
	switch framework {
	case "gin":
		log.Println("Starting server with Gin framework...")
		use_gin.StartServer()
	case "echo":
		log.Println("Starting server with Echo framework...")
		use_echo.StartServer()
	case "mux":
		log.Println("Starting server with Gorilla Mux framework...")
		use_mux.StartServer()
	case "http":
		log.Println("Starting server with standard HTTP library...")
		use_http.StartServer()
	default:
		fmt.Printf("Invalid framework: %s\n", framework)
		fmt.Println("Available frameworks: gin, echo, mux, http")
		os.Exit(1)
	}
}
