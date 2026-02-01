package tools

import (
	"github.com/aistudiolabx/eino-demo/backend/mcp_server/handlers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// All 返回所有 MCP tool 定义及 handler，供 server 一次性注册（AddTools）
func All() []server.ServerTool {
	return []server.ServerTool{
		{
			Tool: mcp.NewTool(
				"weather",
				mcp.WithDescription("查询指定城市的当前天气（使用 Open-Meteo）"),
				mcp.WithString("city", mcp.Required(), mcp.Description("城市名，例如：Beijing、Shenzhen")),
			),
			Handler: server.ToolHandlerFunc(handlers.Weather),
		},
		{
			Tool: mcp.NewTool(
				"novel_to_script",
				mcp.WithDescription("小说转剧本：将小说文本提交至 RunningHub 小说转剧本工作流，自动创建任务、轮询完成并返回剧本结果。API Key 从环境变量 RUNNINGHUB_API_KEY 读取。"),
				mcp.WithString("text", mcp.Required(), mcp.Description("小说正文内容")),
				mcp.WithString("seed", mcp.Description("可选，随机种子，不传则使用默认")),
			),
			Handler: server.ToolHandlerFunc(handlers.NovelToScript),
		},
	}
}
