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

type ProviderConfigs struct {
	Gemini   GeminiConfig   `yaml:"gemini"`
	Doubao   DoubaoConfig   `yaml:"doubao"`
	Deepseek DeepseekConfig `yaml:"deepseek"`
}

// Config 结构体现在包含提示词和 SMTP 分组
type Config struct {
	ActiveProvider string                `yaml:"active_provider"`
	Providers      ProviderConfigs       `yaml:"providers"`
	Prompts        map[string]string     `yaml:"prompts"`     // 新增：预设提示词
	SMTPGroups     map[string]SMTPConfig `yaml:"smtp_groups"` // 修改：支持多 SMTP 配置
	Templates      map[string]string     `yaml:"templates"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
