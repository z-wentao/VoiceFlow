package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// Config 应用配置
type Config struct {
	OpenAI         OpenAIConfig         `yaml:"openai"`
	Transcriber    TranscriberConfig    `yaml:"transcriber"`
	Queue          QueueConfig          `yaml:"queue"`
	Storage        StorageConfig        `yaml:"storage"`
	Server         ServerConfig         `yaml:"server"`
	MaimemoService MaimemoServiceConfig `yaml:"maimemo_service"` // Maimemo 微服务配置
}

// OpenAIConfig OpenAI 配置
type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
}

// TranscriberConfig 转换器配置
type TranscriberConfig struct {
	WorkerPoolSize     int `yaml:"worker_pool_size"`     // Worker 实例数量（同时处理多少个音频文件）
	SegmentConcurrency int `yaml:"segment_concurrency"`  // 每个音频文件的分片并发处理数
	SegmentDuration    int `yaml:"segment_duration"`
	MaxRetries         int `yaml:"max_retries"`
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

// StorageConfig 存储配置
type StorageConfig struct {
	Type     string         `yaml:"type"`     // 存储类型: memory/redis/postgres/hybrid
	Redis    RedisConfig    `yaml:"redis"`    // Redis 配置
	Postgres PostgresConfig `yaml:"postgres"` // PostgreSQL 配置
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`     // Redis 地址，如 "localhost:6379"
	Password string `yaml:"password"` // 密码，无密码留空
	DB       int    `yaml:"db"`       // 数据库编号，默认 0
	TTL      int    `yaml:"ttl"`      // 数据过期时间（小时），默认 168（7天）
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host     string `yaml:"host"`     // 主机地址
	Port     int    `yaml:"port"`     // 端口
	User     string `yaml:"user"`     // 用户名
	Password string `yaml:"password"` // 密码
	Database string `yaml:"database"` // 数据库名
	SSLMode  string `yaml:"sslmode"`  // SSL模式: disable/require/verify-ca/verify-full
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port          int   `yaml:"port"`
	MaxUploadSize int64 `yaml:"max_upload_size"`
}

// MaimemoServiceConfig Maimemo 微服务配置
type MaimemoServiceConfig struct {
	URL     string `yaml:"url"`     // Maimemo 微服务地址
	Timeout int    `yaml:"timeout"` // 超时时间（秒）
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

	if c.Transcriber.SegmentConcurrency <= 0 {
		c.Transcriber.SegmentConcurrency = 3 // 默认 3 个并发分片处理
	}

	if c.Transcriber.SegmentDuration <= 0 {
		c.Transcriber.SegmentDuration = 600
	}

	if c.Server.Port <= 0 {
		c.Server.Port = 8080
	}

	// 存储配置默认值
	if c.Storage.Type == "" {
		c.Storage.Type = "memory"
	}

	// Redis 配置默认值
	if c.Storage.Type == "redis" || c.Storage.Type == "hybrid" {
		if c.Storage.Redis.Addr == "" {
			c.Storage.Redis.Addr = "localhost:6379"
		}
		if c.Storage.Redis.TTL <= 0 {
			c.Storage.Redis.TTL = 168 // 默认 7 天
		}
	}

	// PostgreSQL 配置默认值
	if c.Storage.Type == "postgres" || c.Storage.Type == "hybrid" {
		if c.Storage.Postgres.Host == "" {
			c.Storage.Postgres.Host = "localhost"
		}
		if c.Storage.Postgres.Port <= 0 {
			c.Storage.Postgres.Port = 5432
		}
		if c.Storage.Postgres.SSLMode == "" {
			c.Storage.Postgres.SSLMode = "disable"
		}
	}

	// 队列配置默认值
	if c.Queue.Type == "" {
		c.Queue.Type = "memory"
	}
	if c.Queue.BufferSize <= 0 {
		c.Queue.BufferSize = 100
	}

	// RabbitMQ 配置验证
	if c.Queue.Type == "rabbitmq" {
		if c.Queue.RabbitMQ.URL == "" {
			c.Queue.RabbitMQ.URL = "amqp://guest:guest@localhost:5672/"
		}
		if c.Queue.RabbitMQ.QueueName == "" {
			c.Queue.RabbitMQ.QueueName = "voiceflow_transcription"
		}
	}

	// Maimemo 微服务配置默认值
	if c.MaimemoService.URL == "" {
		c.MaimemoService.URL = "http://localhost:8081"
	}
	if c.MaimemoService.Timeout <= 0 {
		c.MaimemoService.Timeout = 30
	}

	return nil
}
