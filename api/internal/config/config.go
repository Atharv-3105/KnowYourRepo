package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Server ServerConfig
	Extractor ExtractorConfig
	DB 		DBConfig
	CORS    CORSConfig
}

type ServerConfig struct {
	Host   string
	Port   string
}

type ExtractorConfig struct {
	BaseURL  string
}

type DBConfig struct {
	DSN   string
}

type CORSConfig struct {
	AllowedOrigins []string
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}


func Load() (*Config, error) {
	cfg := &Config{
		Server:  ServerConfig{
			Host:   getEnv("SERVER_HOST", "0.0.0.0"),
			Port:   getEnv("SERVER_PORT", "8080"),
		},
		Extractor: ExtractorConfig{
			BaseURL: getEnv("EXTRACTOR_URL", "http://localhost:8000"),
		},
		DB:  DBConfig{
			DSN:   getEnv("DATABASE_URL", "postgres://postgres:atharva@localhost:5432/knowyourrepo?sslmode=disable"),
		},
		CORS: CORSConfig{
			AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"), ","),
		},
	}
	return cfg, nil
}


func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback 
}