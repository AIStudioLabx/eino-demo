package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://www.runninghub.ai"

// 轮询间隔与默认超时
const pollInterval = 2 * time.Second
const defaultRunTimeout = 10 * time.Minute

// NodeInfo ComfyUI 节点参数
type NodeInfo struct {
	NodeID     string `json:"nodeId"`
	FieldName  string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	APIKey       string     `json:"apiKey"`
	WorkflowID   string     `json:"workflowId"`
	NodeInfoList []NodeInfo `json:"nodeInfoList"`
}

// TaskRequest 状态/输出请求（共用）
type TaskRequest struct {
	APIKey string `json:"apiKey"`
	TaskID string `json:"taskId"`
}

// RunningHubClient RunningHub OpenAPI 客户端
type RunningHubClient struct {
	HTTPClient *http.Client
}

// NewRunningHubClient 创建 RunningHub 客户端
func NewRunningHubClient() *RunningHubClient {
	return &RunningHubClient{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Post 发送 POST JSON 请求，返回响应体和状态码
func (c *RunningHubClient) Post(path string, body any) ([]byte, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "www.runninghub.ai")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, resp.StatusCode, err
	}
	return buf.Bytes(), resp.StatusCode, nil
}

// CreateTask 创建任务
func (c *RunningHubClient) CreateTask(apiKey, workflowID string, nodeInfoList []NodeInfo) ([]byte, int, error) {
	return c.Post("/task/openapi/create", CreateTaskRequest{
		APIKey:       apiKey,
		WorkflowID:   workflowID,
		NodeInfoList: nodeInfoList,
	})
}

// TaskStatus 查询任务状态
func (c *RunningHubClient) TaskStatus(apiKey, taskID string) ([]byte, int, error) {
	return c.Post("/task/openapi/status", TaskRequest{APIKey: apiKey, TaskID: taskID})
}

// TaskOutputs 获取任务输出
func (c *RunningHubClient) TaskOutputs(apiKey, taskID string) ([]byte, int, error) {
	return c.Post("/task/openapi/outputs", TaskRequest{APIKey: apiKey, TaskID: taskID})
}

// TaskOutputsResponse 获取输出 API 响应
// 示例：{ "code": 0, "msg": "success", "data": [ { "fileUrl": "https://...", "fileType": "txt", "nodeId": "9", "taskCostTime": "18", ... } ] }
type TaskOutputsResponse struct {
	Code          int               `json:"code"`
	Msg           string            `json:"msg"`
	ErrorMessages any               `json:"errorMessages,omitempty"`
	Data          []TaskOutputsItem `json:"data"`
}

// TaskOutputsItem 输出项（含下载链接等）
type TaskOutputsItem struct {
	FileUrl                string `json:"fileUrl"`
	FileType               string `json:"fileType"`
	NodeID                 string `json:"nodeId"`
	TaskCostTime           string `json:"taskCostTime"`
	ThirdPartyConsumeMoney any    `json:"thirdPartyConsumeMoney,omitempty"`
	ConsumeMoney           any    `json:"consumeMoney,omitempty"`
	ConsumeCoins           string `json:"consumeCoins,omitempty"`
}

// FetchOutputTextContent 解析 outputs API 的 JSON 响应，下载 data 中 fileType 为 txt 的 fileUrl 内容并拼接返回
func (c *RunningHubClient) FetchOutputTextContent(outputsJSON []byte) (string, error) {
	var resp TaskOutputsResponse
	if err := json.Unmarshal(outputsJSON, &resp); err != nil {
		return "", fmt.Errorf("解析输出响应失败: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("输出 API 返回 code=%d msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("输出 API 无 data")
	}
	var sb strings.Builder
	httpClient := &http.Client{Timeout: 30 * time.Second}
	for i, item := range resp.Data {
		if item.FileUrl == "" {
			continue
		}
		// 优先下载 fileType 为 txt 的项；若没有 txt 则下载所有有 fileUrl 的项
		if item.FileType != "" && item.FileType != "txt" {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, item.FileUrl, nil)
		if err != nil {
			return "", fmt.Errorf("创建下载请求失败: %w", err)
		}
		r, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("下载 %s 失败: %w", item.FileUrl, err)
		}
		if r.StatusCode != http.StatusOK {
			r.Body.Close()
			return "", fmt.Errorf("下载 %s 返回 %d", item.FileUrl, r.StatusCode)
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return "", fmt.Errorf("读取 %s 内容失败: %w", item.FileUrl, err)
		}
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(strings.TrimSpace(string(body)))
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("输出 API 中无 txt 类型文件")
	}
	return sb.String(), nil
}

// CreateTaskResponse 创建任务 API 响应
// 示例：{ "code": 0, "msg": "success", "data": { "taskId": "xxx", "taskStatus": "RUNNING", "netWssUrl": "...", "clientId": "...", "promptTips": "..." } }
type CreateTaskResponse struct {
	Code          int    `json:"code"`
	Msg           string `json:"msg"`
	ErrorMessages any    `json:"errorMessages,omitempty"`
	Data          struct {
		TaskID     string `json:"taskId"`
		TaskStatus string `json:"taskStatus"`
		NetWssUrl  string `json:"netWssUrl,omitempty"`
		ClientID   string `json:"clientId,omitempty"`
		PromptTips string `json:"promptTips,omitempty"`
	} `json:"data"`
}

// TaskStatusResponse 任务状态 API 响应
// 示例：{ "code": 0, "msg": "success", "data": "SUCCESS" }（data 为字符串）；或 data 为对象时取 taskStatus
type TaskStatusResponse struct {
	Code          int             `json:"code"`
	Msg           string          `json:"msg"`
	ErrorMessages any             `json:"errorMessages,omitempty"`
	Data          json.RawMessage `json:"data"` // "SUCCESS" | "RUNNING" | "FAILED" 或对象 { "taskStatus": "..." }
}

// parseTaskStatus 从状态 API 的 data 字段解析出状态字符串（data 可能为 JSON 字符串或对象）
func parseTaskStatus(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return s
	}
	var obj struct {
		TaskStatus string `json:"taskStatus"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		return obj.TaskStatus
	}
	return ""
}

// RunWorkflow 创建任务并轮询直至完成，最后返回输出结果；可被业务层复用。
// 状态值参考 RunningHub：QUEUED、RUNNING、SUCCESS、FAILED。
func (c *RunningHubClient) RunWorkflow(ctx context.Context, apiKey, workflowID string, nodeInfoList []NodeInfo) (outputs []byte, err error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRunTimeout)
		defer cancel()
	}

	raw, code, err := c.CreateTask(apiKey, workflowID, nodeInfoList)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("创建任务 API 返回 %d: %s", code, string(raw))
	}
	var createResp CreateTaskResponse
	if err := json.Unmarshal(raw, &createResp); err != nil {
		return nil, fmt.Errorf("解析创建响应失败: %w", err)
	}
	taskID := createResp.Data.TaskID
	if taskID == "" {
		return nil, fmt.Errorf("创建响应中缺少 taskId: %s", string(raw))
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		raw, code, err := c.TaskStatus(apiKey, taskID)
		if err != nil {
			return nil, fmt.Errorf("查询状态失败: %w", err)
		}
		if code != http.StatusOK {
			return nil, fmt.Errorf("状态 API 返回 %d: %s", code, string(raw))
		}
		var statusResp TaskStatusResponse
		if err := json.Unmarshal(raw, &statusResp); err != nil {
			return nil, fmt.Errorf("解析状态响应失败: %w", err)
		}
		status := parseTaskStatus(statusResp.Data)
		switch status {
		case "SUCCESS":
			out, code, err := c.TaskOutputs(apiKey, taskID)
			if err != nil {
				return nil, fmt.Errorf("获取输出失败: %w", err)
			}
			if code != http.StatusOK {
				return nil, fmt.Errorf("输出 API 返回 %d: %s", code, string(out))
			}
			return out, nil
		case "FAILED":
			return nil, fmt.Errorf("任务执行失败: %s", string(raw))
		case "QUEUED", "RUNNING", "":
			continue
		default:
			continue
		}
	}
}
