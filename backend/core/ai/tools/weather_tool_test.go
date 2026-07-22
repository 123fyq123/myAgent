package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWeatherToolInfoAndParams(t *testing.T) {
	weatherTool := NewWeatherTool(&WeatherConfig{
		ApiKey: "test-key",
	})
	info, err := weatherTool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.Name != "get_weather" {
		t.Fatalf("Info().Name = %q, want get_weather", info.Name)
	}
	if len(weatherTool.Params()) == 0 {
		t.Fatalf("Params() should not be empty")
	}
}

func TestWeatherToolRequiresCity(t *testing.T) {
	weatherTool := NewWeatherTool(&WeatherConfig{
		ApiKey: "test-key",
	})
	params := map[string]string{
		"extensions": "all",
	}
	marshal, _ := json.Marshal(params)
	_, err := weatherTool.InvokableRun(context.Background(), string(marshal))
	if err == nil {
		t.Fatal("InvokableRun() should require city")
	}
}
