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

		// 系统提示：要求模型在用户请求天气或小说转剧本时必须调用对应工具，避免只返回纯文本导致 Tools 节点报错
		systemPrompt := `你是一个具备工具调用能力的助手。请根据用户意图调用对应工具，不要仅用文字回复。

- 当用户询问某地天气、城市天气时，你必须调用 weather 工具，参数 city 填城市名（如 Beijing、上海）。
- 当用户要求将小说转成剧本、或提供小说正文要转换时，你必须调用 novel_to_script 工具，参数 text 填小说正文或用户提供的文本，可选参数 seed 可填数字字符串。
- 以上场景下必须先调用工具，再根据工具返回结果组织回复；不要不调用工具而直接文字回答。
- 特别地，当 novel_to_script 工具返回后，你的最终回复只输出工具返回的剧本文本内容本身，不要加任何总结、开场白、结束语或链接说明（例如不要写 "Great! Here is the script:" 或 "You can download..." 等），只输出剧本正文。`

		msgs := []*schema.Message{
			schema.SystemMessage(systemPrompt),
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
