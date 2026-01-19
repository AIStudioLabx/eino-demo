package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	// 静态文件目录：项目根下的 frontend
	// 建议在项目根目录执行：go run ./backend/frontend_server
	fs := http.FileServer(http.Dir("frontend"))

	// 使用根路由提供前端：访问 http://localhost:8081 即返回 index.html
	mux.Handle("/", fs)

	addr := ":8081"
	log.Printf("Frontend server listening on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("frontend server error: %v", err)
	}
}

