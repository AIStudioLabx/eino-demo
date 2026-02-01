package handlers

import (
	"context"
	"fmt"

	"github.com/aistudiolabx/eino-demo/backend/mcp_server/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var openMeteoClient = client.NewOpenMeteoClient()

// Weather 查询指定城市当前天气的 MCP tool handler
func Weather(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	city, err := req.RequireString("city")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	geo, err := openMeteoClient.Geocode(city)
	if err != nil {
		return mcp.NewToolResultError("地理编码: " + err.Error()), nil
	}
	if len(geo.Results) == 0 {
		return mcp.NewToolResultError("未找到该城市的地理信息"), nil
	}

	r := geo.Results[0]
	lat, lon := r.Latitude, r.Longitude
	displayName := r.Name
	if r.Country != "" {
		displayName = fmt.Sprintf("%s, %s", r.Name, r.Country)
	}

	w, err := openMeteoClient.GetWeather(lat, lon)
	if err != nil {
		return mcp.NewToolResultError("天气查询: " + err.Error()), nil
	}

	desc := client.WeatherCodeToDesc(w.Current.WeatherCode)
	result := fmt.Sprintf("城市: %s，经纬度: (%.3f, %.3f)，温度: %.1f°C，天气: %s",
		displayName, lat, lon, w.Current.Temperature2m, desc)
	return mcp.NewToolResultText(result), nil
}
