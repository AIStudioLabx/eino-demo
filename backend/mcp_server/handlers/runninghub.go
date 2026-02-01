package handlers

import (
	"context"
	"fmt"
	"os"

	"github.com/aistudiolabx/eino-demo/backend/mcp_server/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const runningHubAPIKeyEnv = "RUNNINGHUB_API_KEY"

var runningHubClient = client.NewRunningHubClient()

func getRunningHubAPIKey() (string, error) {
	apiKey := os.Getenv(runningHubAPIKeyEnv)
	if apiKey == "" {
		return "", fmt.Errorf("未配置环境变量 %s", runningHubAPIKeyEnv)
	}
	return apiKey, nil
}

// 小说转剧本工作流 ID（RunningHub）
const novelToScriptWorkflowID = "2014935539987783681"

// 小说转剧本工作流节点：节点 8 为文本输入，节点 6 为 seed（可选）
const novelToScriptNodeText = "8"
const novelToScriptNodeSeed = "6"

// NovelToScript 小说转剧本：创建任务、轮询完成并返回结果（业务级 MCP tool）
func NovelToScript(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey, err := getRunningHubAPIKey()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	nodeInfoList := []client.NodeInfo{
		{NodeID: novelToScriptNodeText, FieldName: "text", FieldValue: text},
	}
	if seed := req.GetString("seed", ""); seed != "" {
		nodeInfoList = append(nodeInfoList, client.NodeInfo{
			NodeID: novelToScriptNodeSeed, FieldName: "seed", FieldValue: seed,
		})
	}
	outputs, err := runningHubClient.RunWorkflow(ctx, apiKey, novelToScriptWorkflowID, nodeInfoList)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	// 解析 outputs 中的 fileUrl，下载 txt 文件内容并作为返回
	content, err := runningHubClient.FetchOutputTextContent(outputs)
	if err != nil {
		return mcp.NewToolResultError("下载输出文件失败: " + err.Error()), nil
	}
	return mcp.NewToolResultText(content), nil
}
