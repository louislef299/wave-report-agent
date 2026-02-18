package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"google.golang.org/adk/tool"
)

// https://open-meteo.com/en/docs/marine-weather-api#data_sources

type OpenMeteoResp struct {
	HourlyUnits HourlyUnits `json:"hourly_units"`
	Hourly      Hourly      `json:"hourly"`
}

type HourlyUnits struct {
	Time               string `json:"time"`
	WaveHeight         string `json:"wave_height"`
	WaveDirection      string `json:"wave_direction"`
	WavePeriod         string `json:"wave_period"`
	WindWaveHeight     string `json:"wind_wave_height"`
	WindWaveDirection  string `json:"wind_wave_direction"`
	WindWavePeriod     string `json:"wind_wave_period"`
	SwellWaveHeight    string `json:"swell_wave_height"`
	SwellWaveDirection string `json:"swell_wave_direction"`
	SwellWavePeriod    string `json:"swell_wave_period"`
	SeaLevelHeightMsl  string `json:"sea_level_height_msl"`
}

type Hourly struct {
	Time               []string  `json:"time"`
	WaveHeight         []float32 `json:"wave_height"`
	WaveDirection      []int32   `json:"wave_direction"`
	WavePeriod         []float32 `json:"wave_period"`
	WindWaveHeight     []float32 `json:"wind_wave_height"`
	WindWaveDirection  []int32   `json:"wind_wave_direction"`
	WindWavePeriod     []float32 `json:"wind_wave_period"`
	SwellWaveHeight    []float32 `json:"swell_wave_height"`
	SwellWaveDirection []int32   `json:"swell_wave_direction"`
	SwellWavePeriod    []float32 `json:"swell_wave_period"`
	SeaLevelHeightMsl  []float32 `json:"sea_level_height_msl"`
}

func GetHourlyMarineForecast(ctx tool.Context, s *spot.Spot) (*OpenMeteoResp, error) {
	resp, err := http.Get(generateMarineUrl(s))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidHttpResponse
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var openResp OpenMeteoResp
	err = json.Unmarshal(resBody, &openResp)
	if err != nil {
		return nil, err
	}
	return &openResp, nil
}

func generateMarineUrl(s *spot.Spot) string {
	return fmt.Sprintf("https://marine-api.open-meteo.com/v1/marine?latitude=%.2f&longitude=%.2f&hourly=wave_height,wave_direction,wave_period,wind_wave_height,wind_wave_direction,wind_wave_period,swell_wave_height,swell_wave_direction,swell_wave_period,sea_level_height_msl&length_unit=imperial&wind_speed_unit=kn", s.Latitude, s.Longitude)
}
