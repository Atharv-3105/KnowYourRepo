package config

import (
	"fmt"
	"os"
)

type Config struct {
	Server ServerConfig 
	Extractor ExtractorConfig
	DB 		DBConfig
}

type ServerConfig struct {
	Host   string 
	Port   string 
}

type ExtractorConfig struct {
	BaseURL  string 
}

type DBConfig struct {
	Path   string 
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
			Path:   getEnv("DB_PATH", "./data/codebase.db"),
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