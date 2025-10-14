package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// Config 应用配置
type Config struct {
	OpenAI      OpenAIConfig      `yaml:"openai"`
	Transcriber TranscriberConfig `yaml:"transcriber"`
	Queue       QueueConfig       `yaml:"queue"`
	Server      ServerConfig      `yaml:"server"`
}

// OpenAIConfig OpenAI 配置
type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
}

// TranscriberConfig 转换器配置
type TranscriberConfig struct {
	WorkerPoolSize  int `yaml:"worker_pool_size"`  // Worker 实例数量（同时处理多少个音频文件）
	WorkerCount     int `yaml:"worker_count"`      // 每个音频文件的并发分段数
	SegmentDuration int `yaml:"segment_duration"`
	MaxRetries      int `yaml:"max_retries"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Type       string          `yaml:"type"`
	BufferSize int             `yaml:"buffer_size"`
	RabbitMQ   RabbitMQConfig  `yaml:"rabbitmq"`
}

// RabbitMQConfig RabbitMQ 配置
type RabbitMQConfig struct {
	URL       string `yaml:"url"`
	QueueName string `yaml:"queue_name"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port          int   `yaml:"port"`
	MaxUploadSize int64 `yaml:"max_upload_size"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析 YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	return &config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.OpenAI.APIKey == "" || c.OpenAI.APIKey == "your-openai-api-key-here" {
		return fmt.Errorf("请在配置文件中设置有效的 OpenAI API Key")
	}

	if c.Transcriber.WorkerPoolSize <= 0 {
		c.Transcriber.WorkerPoolSize = 2 // 默认 2 个 Worker 实例
	}

	if c.Transcriber.WorkerCount <= 0 {
		c.Transcriber.WorkerCount = 3
	}

	if c.Transcriber.SegmentDuration <= 0 {
		c.Transcriber.SegmentDuration = 600
	}

	if c.Server.Port <= 0 {
		c.Server.Port = 8080
	}

	return nil
}
