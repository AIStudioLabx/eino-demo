package main

import (
	"context"
	"os"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

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
