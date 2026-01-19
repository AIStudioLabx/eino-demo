package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Open-Meteo 地理编码返回结构
type openMeteoGeocodeResp struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
	} `json:"results"`
}

// Open-Meteo 当前天气返回结构
type openMeteoWeatherResp struct {
	Current struct {
		Temperature2m float64 `json:"temperature_2m"`
		WeatherCode   int     `json:"weather_code"`
	} `json:"current"`
}

func weatherCodeToDesc(code int) string {
	switch code {
	case 0:
		return "晴朗"
	case 1, 2, 3:
		return "多云"
	case 45, 48:
		return "有雾"
	case 51, 53, 55, 56, 57:
		return "小雨"
	case 61, 63, 65:
		return "降雨"
	case 71, 73, 75:
		return "降雪"
	case 95, 96, 99:
		return "雷暴"
	default:
		return "未知天气"
	}
}

func weatherToolHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	city, err := req.RequireString("city")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// 1. 先用 Open-Meteo geocoding 查城市经纬度
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json", city)
	geoResp, err := httpClient.Get(geoURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("geocoding http error: %v", err)), nil
	}
	defer geoResp.Body.Close()

	if geoResp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("geocoding API status: %s", geoResp.Status)), nil
	}

	var geo openMeteoGeocodeResp
	if err := json.NewDecoder(geoResp.Body).Decode(&geo); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("geocoding decode error: %v", err)), nil
	}
	if len(geo.Results) == 0 {
		return mcp.NewToolResultError("未找到该城市的地理信息"), nil
	}

	lat := geo.Results[0].Latitude
	lon := geo.Results[0].Longitude
	displayName := geo.Results[0].Name
	if geo.Results[0].Country != "" {
		displayName = fmt.Sprintf("%s, %s", displayName, geo.Results[0].Country)
	}

	// 2. 查询 Open-Meteo 当前天气
	weatherURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,weather_code",
		lat, lon,
	)

	wResp, err := httpClient.Get(weatherURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("weather http error: %v", err)), nil
	}
	defer wResp.Body.Close()

	if wResp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("weather API status: %s", wResp.Status)), nil
	}

	var w openMeteoWeatherResp
	if err := json.NewDecoder(wResp.Body).Decode(&w); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("weather decode error: %v", err)), nil
	}

	desc := weatherCodeToDesc(w.Current.WeatherCode)
	result := fmt.Sprintf("城市: %s，经纬度: (%.3f, %.3f)，温度: %.1f°C，天气: %s",
		displayName, lat, lon, w.Current.Temperature2m, desc)

	return mcp.NewToolResultText(result), nil
}

func main() {
	s := server.NewMCPServer("weather_agent", "1.0.0", server.WithToolCapabilities(false))

	tool := mcp.NewTool(
		"weather",
		mcp.WithDescription("查询指定城市的当前天气（使用 Open-Meteo）"),
		mcp.WithString(
			"city",
			mcp.Required(),
			mcp.Description("城市名，例如：Beijing、Shenzhen"),
		),
	)

	s.AddTool(tool, weatherToolHandler)

	// 使用原生 SSE Server，监听 :3333，并在 /sse 上提供 SSE，在 /message 上收发消息
	addr := ":3333"
	log.Printf("MCP Weather SSE server listening on %s\n", addr)

	sseServer := server.NewSSEServer(
		s,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)

	if err := sseServer.Start(addr); err != nil {
		log.Fatalf("mcp sse server error: %v", err)
	}
}
