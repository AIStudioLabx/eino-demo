package main

type agentRequest struct {
	Input string `json:"input"`
}

type agentResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}
