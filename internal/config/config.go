package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type SMTPConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	FromAlias string `yaml:"from_alias"`
}

// ... ProviderConfigs 结构体保持不变 ...
type ProviderConfigs struct {
	Gemini   GeminiConfig   `yaml:"gemini"`
	Doubao   DoubaoConfig   `yaml:"doubao"`
	Deepseek DeepseekConfig `yaml:"deepseek"`
}
type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}
type DoubaoConfig struct {
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
}
type DeepseekConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// 新增：发送策略的结构体
type SendingStrategy struct {
	Policy   string   `yaml:"policy"`   // "round-robin", "random"
	Accounts []string `yaml:"accounts"` // 引用 smtp_accounts 中的 key
}

// Config 结构体大幅更新
type Config struct {
	ActiveProvider         string                     `yaml:"active_provider"`
	Providers              ProviderConfigs            `yaml:"providers"`
	Prompts                map[string]string          `yaml:"prompts"`
	StructuredInstructions map[string]string          `yaml:"structured_instructions"` // 新增
	SMTPAccounts           map[string]SMTPConfig      `yaml:"smtp_accounts"`           // 新增
	SendingStrategies      map[string]SendingStrategy `yaml:"sending_strategies"`      // 新增
	Templates              map[string]string          `yaml:"templates"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
