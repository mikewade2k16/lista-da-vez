// Package observability coleta métricas de requisição, banco, chamadas externas e
// eventos do front, gravando de forma assíncrona no schema obs do Postgres.
//
// Tudo é desligável por OBSERVABILITY_ENABLED=false: nesse caso o Runtime é nil-safe
// e nenhum middleware, tracer ou job grava dados.
package observability

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Enabled            bool
	SampleRate         float64
	SlowQuery          time.Duration
	SlowRequest        time.Duration
	NPlusOneThreshold  int
	RetentionDays      int
	BufferSize         int
	FlushInterval      time.Duration
	MaintenanceEvery   time.Duration
	Environment        string
	MaxStatementLength int
}

func ConfigFromEnv(environment string) Config {
	return Config{
		Enabled:            envBool("OBSERVABILITY_ENABLED", true),
		SampleRate:         clampRate(envFloat("OBS_SAMPLE_RATE", 1)),
		SlowQuery:          time.Duration(envInt("OBS_SLOW_QUERY_MS", 200)) * time.Millisecond,
		SlowRequest:        time.Duration(envInt("OBS_SLOW_REQUEST_MS", 1000)) * time.Millisecond,
		NPlusOneThreshold:  max(2, envInt("OBS_NPLUSONE_THRESHOLD", 10)),
		RetentionDays:      max(1, envInt("OBS_RETENTION_DAYS", 30)),
		BufferSize:         max(100, envInt("OBS_BUFFER_SIZE", 5000)),
		FlushInterval:      5 * time.Second,
		MaintenanceEvery:   time.Hour,
		Environment:        strings.TrimSpace(environment),
		MaxStatementLength: 2000,
	}
}

// IsProduction indica ambiente produtivo; o painel não deixa disparar o runner nele.
func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production") || strings.EqualFold(c.Environment, "prod")
}

func clampRate(rate float64) float64 {
	switch {
	case rate < 0:
		return 0
	case rate > 1:
		return 1
	default:
		return rate
	}
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil {
		return fallback
	}
	return value
}
