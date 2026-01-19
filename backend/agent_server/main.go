package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	ollama "github.com/cloudwego/eino-ext/components/model/ollama"
	mcpTool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type agentRequest struct {
	Input string `json:"input"`
}

type agentResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func newMCPClient(ctx context.Context) (client.MCPClient, error) {
	// 默认连本地的 MCP Weather server
	endpoint := os.Getenv("MCP_WEATHER_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:3333/sse"
	}

	mcpClient, err := client.NewSSEMCPClient(endpoint)
	if err != nil {
		return nil, err
	}

	// 启动 SSE 客户端
	if err := mcpClient.Start(ctx); err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "eino-weather-agent",
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		return nil, err
	}

	return mcpClient, nil
}

func buildAgent(ctx context.Context) (compose.Runnable[[]*schema.Message, []*schema.Message], error) {
	cli, err := newMCPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("newMCPClient: %w", err)
	}

	tools, err := mcpTool.GetTools(ctx, &mcpTool.Config{Cli: cli})
	if err != nil {
		return nil, fmt.Errorf("GetTools: %w", err)
	}

	// 使用本地 Ollama 作为 ChatModel
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   "qwen2.5:7b",
	})
	if err != nil {
		return nil, fmt.Errorf("NewChatModel(ollama): %w", err)
	}

	// 绑定工具信息
	var toolInfos []*schema.ToolInfo
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("tool.Info: %w", err)
		}
		toolInfos = append(toolInfos, info)
	}
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("BindTools: %w", err)
	}

	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: tools,
	})
	if err != nil {
		return nil, fmt.Errorf("NewToolNode: %w", err)
	}

	chain := compose.NewChain[[]*schema.Message, []*schema.Message]()
	chain.
		AppendChatModel(chatModel, compose.WithNodeName("chat_model")).
		AppendToolsNode(toolsNode, compose.WithNodeName("tools")).
		AppendChatModel(chatModel, compose.WithNodeName("chat_model_final")).
		AppendLambda(compose.ToList[*schema.Message]())

	return chain.Compile(ctx)
}

func main() {
	ctx := context.Background()

	agent, err := buildAgent(ctx)
	if err != nil {
		log.Fatalf("buildAgent error: %v", err)
	}

	mux := http.NewServeMux()

	// 简单的健康检查
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// agent 调用接口（带简单 CORS 支持，允许前端在 8081 访问）
	mux.HandleFunc("/agent", func(w http.ResponseWriter, r *http.Request) {
		// CORS 头
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8081")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// 预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req agentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Input == "" {
			http.Error(w, "input is required", http.StatusBadRequest)
			return
		}

		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: req.Input,
			},
		}

		respMsgs, err := agent.Invoke(ctx, msgs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(agentResponse{Error: err.Error()})
			return
		}

		// 优先取 assistant 的回复；如果没有，则退回最后一条消息的内容（通常包含 tool 结果）
		var out string
		for _, m := range respMsgs {
			if m.Role == schema.Assistant {
				out = m.Content
				break
			}
		}
		if out == "" && len(respMsgs) > 0 {
			out = respMsgs[len(respMsgs)-1].Content
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(agentResponse{Output: out})
	})

	addr := ":8082"
	log.Printf("Agent server listening on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("agent server error: %v", err)
	}
}
