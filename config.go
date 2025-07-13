package main

import (
	"encoding/json"
	"os"
)

type AIConfig struct {
	APIKey string `json:"api_key"`
}

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	AIConfig   AIConfig   `json:"ai_config"`
	SMTPConfig SMTPConfig `json:"smtp_config"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}