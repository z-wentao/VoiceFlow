package models

import "time"

// JobStatus 任务状态枚举
type JobStatus string

const (
	StatusPending    JobStatus = "pending"     // 等待处理
	StatusProcessing JobStatus = "processing"  // 处理中
	StatusCompleted  JobStatus = "completed"   // 已完成
	StatusFailed     JobStatus = "failed"      // 失败
)

// WordDetail 单词详细信息
type WordDetail struct {
	Word       string `json:"word"`       // 单词
	Definition string `json:"definition"` // 释义
	Example    string `json:"example"`    // 例句
}

// TranscriptionJob 转换任务
type TranscriptionJob struct {
	JobID       string       `json:"job_id"`        // 任务ID
	Filename    string       `json:"filename"`      // 原始文件名
	FilePath    string       `json:"file_path"`     // 文件存储路径
	Status      JobStatus    `json:"status"`        // 任务状态
	Progress    int          `json:"progress"`      // 进度 0-100
	Result      string       `json:"result"`        // 转换结果文本
	Language    string       `json:"language"`      // 识别的语言
	Duration    float64      `json:"duration"`      // 音频时长（秒）
	Error       string       `json:"error"`         // 错误信息
	Vocabulary  []string     `json:"vocabulary"`    // 提取的单词列表（仅单词）
	VocabDetail []WordDetail `json:"vocab_detail"`  // 单词详细信息
	CreatedAt   time.Time    `json:"created_at"`    // 创建时间
	CompletedAt time.Time    `json:"completed_at"`  // 完成时间

	// RabbitMQ 相关（不序列化到 JSON）
	DeliveryTag      uint64      `json:"-"` // RabbitMQ delivery tag
	RabbitMQDelivery interface{} `json:"-"` // RabbitMQ delivery 对象（用于 Ack/Nack）
}

// Segment 音频片段
type Segment struct {
	Index    int     `json:"index"`     // 片段序号
	FilePath string  `json:"file_path"` // 片段文件路径
	Start    float64 `json:"start"`     // 开始时间（秒）
	End      float64 `json:"end"`       // 结束时间（秒）
}

// TranscriptionResult 转换结果
type TranscriptionResult struct {
	SegmentIndex int    `json:"segment_index"` // 片段序号
	Text         string `json:"text"`          // 文本内容
	Error        error  `json:"error"`         // 错误
}
