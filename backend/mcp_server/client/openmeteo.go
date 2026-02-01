package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const openMeteoGeocodeURL = "https://geocoding-api.open-meteo.com/v1/search"
const openMeteoForecastURL = "https://api.open-meteo.com/v1/forecast"

// GeocodeResult 地理编码单条结果
type GeocodeResult struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
}

// GeocodeResponse 地理编码 API 响应
type GeocodeResponse struct {
	Results []GeocodeResult `json:"results"`
}

// WeatherCurrent 当前天气
type WeatherCurrent struct {
	Temperature2m float64 `json:"temperature_2m"`
	WeatherCode   int     `json:"weather_code"`
}

// WeatherResponse 天气 API 响应
type WeatherResponse struct {
	Current WeatherCurrent `json:"current"`
}

// OpenMeteoClient Open-Meteo API 客户端
type OpenMeteoClient struct {
	HTTPClient *http.Client
}

// NewOpenMeteoClient 创建 Open-Meteo 客户端
func NewOpenMeteoClient() *OpenMeteoClient {
	return &OpenMeteoClient{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Geocode 根据城市名查询经纬度
func (c *OpenMeteoClient) Geocode(city string) (*GeocodeResponse, error) {
	url := fmt.Sprintf("%s?name=%s&count=1&language=zh&format=json", openMeteoGeocodeURL, city)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API status: %s", resp.Status)
	}
	var out GeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWeather 根据经纬度查询当前天气
func (c *OpenMeteoClient) GetWeather(lat, lon float64) (*WeatherResponse, error) {
	url := fmt.Sprintf("%s?latitude=%f&longitude=%f&current=temperature_2m,weather_code",
		openMeteoForecastURL, lat, lon)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API status: %s", resp.Status)
	}
	var out WeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WeatherCodeToDesc 天气代码转描述
func WeatherCodeToDesc(code int) string {
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
