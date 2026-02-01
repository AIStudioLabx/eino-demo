package main

import (
	"log"

	"github.com/aistudiolabx/eino-demo/backend/mcp_server/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("weather_agent", "1.0.0", server.WithToolCapabilities(false))

	s.AddTools(tools.All()...)

	addr := ":3333"
	log.Printf("MCP SSE server listening on %s\n", addr)

	sseServer := server.NewSSEServer(
		s,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)

	if err := sseServer.Start(addr); err != nil {
		log.Fatalf("mcp sse server error: %v", err)
	}
}
