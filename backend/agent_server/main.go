package main

import (
	"context"
	"log"
	"net/http"
)

func main() {
	ctx := context.Background()

	agent, err := buildAgent(ctx, false)
	if err != nil {
		log.Fatalf("buildAgent error: %v", err)
	}
	toolAgent, err := buildAgent(ctx, true)
	if err != nil {
		log.Fatalf("buildToolAgent error: %v", err)
	}

	mux := http.NewServeMux()

	// 简单的健康检查
	mux.HandleFunc("/healthz", healthzHandler)

	// agent 调用接口（带简单 CORS 支持，允许前端在 8081 访问）
	mux.HandleFunc("/agent", agentHandler(agent))
	mux.HandleFunc("/tool_agent", toolAgentHandler(toolAgent))

	addr := ":8082"
	log.Printf("Agent server listening on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("agent server error: %v", err)
	}
}
