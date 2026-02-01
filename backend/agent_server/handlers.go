package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func agentHandler(agent compose.Runnable[[]*schema.Message, []*schema.Message]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
	}
}

// toolAgentHandler 专门处理 ToolAgent 的响应，从 JSON 格式中提取 content 字段
func toolAgentHandler(agent compose.Runnable[[]*schema.Message, []*schema.Message]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

		// 获取最后一条消息的内容（ToolAgent 返回的是工具结果）
		var out string
		if len(respMsgs) > 0 {
			out = respMsgs[len(respMsgs)-1].Content
		}

		// 尝试解析 JSON 格式，提取 content 字段
		extractedContent := extractContentFromJSON(out)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(agentResponse{Output: extractedContent})
	}
}

// extractContentFromJSON 从 JSON 格式中提取 content 字段的文本内容
// 支持的格式: {"content":[{"type":"text","text":"..."}]}
func extractContentFromJSON(input string) string {
	// 如果输入为空，直接返回
	if strings.TrimSpace(input) == "" {
		return input
	}

	// 尝试解析为 JSON
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal([]byte(input), &result); err != nil {
		// 如果不是有效的 JSON 或格式不匹配，返回原始内容
		return input
	}

	// 提取第一个 content 项的 text 字段
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		return result.Content[0].Text
	}

	// 如果格式不匹配，返回原始内容
	return input
}
