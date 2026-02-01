package main

import (
	"context"
	"fmt"

	ollama "github.com/cloudwego/eino-ext/components/model/ollama"
	mcpTool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func buildAgent(ctx context.Context, isToolAgent bool) (compose.Runnable[[]*schema.Message, []*schema.Message], error) {
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
		AppendToolsNode(toolsNode, compose.WithNodeName("tools"))
	if !isToolAgent {
		chain.AppendChatModel(chatModel, compose.WithNodeName("chat_model_final"))
		chain.AppendLambda(compose.ToList[*schema.Message]())
	}

	return chain.Compile(ctx)
}
