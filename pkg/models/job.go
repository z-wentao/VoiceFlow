package models

import "time"

type JobStatus string

const (
    StatusPending    JobStatus = "pending"     
    StatusProcessing JobStatus = "processing"  
    StatusCompleted  JobStatus = "completed"    
    StatusFailed     JobStatus = "failed"      
)

type WordDetail struct {
    Word       string `json:"word"`       
    Definition string `json:"definition"` 
    Example    string `json:"example"`   
}

type TranscriptionJob struct {
    JobID            string       `json:"job_id"`
    Filename         string       `json:"filename"`
    FilePath         string       `json:"file_path"`
    Status           JobStatus    `json:"status"`
    Progress         int          `json:"progress"`
    Result           string       `json:"result"`
    SubtitlePath     string       `json:"subtitle_path"`          // SRT 字幕文件路径（单语）
    VTTPath          string       `json:"vtt_path"`               // WebVTT 字幕文件路径（单语）
    BilingualSRTPath string       `json:"bilingual_srt_path"`     // 双语 SRT 字幕文件路径
    BilingualVTTPath string       `json:"bilingual_vtt_path"`     // 双语 WebVTT 字幕文件路径
    Language         string       `json:"language"`
    Duration         float64      `json:"duration"`
    Error            string       `json:"error"`
    Vocabulary       []string     `json:"vocabulary"`
    VocabDetail      []WordDetail `json:"vocab_detail"`
    CreatedAt        time.Time    `json:"created_at"`
    CompletedAt      time.Time    `json:"completed_at"`

    // RabbitMQ 相关（不序列化到 JSON）
    DeliveryTag      uint64      `json:"-"` // RabbitMQ delivery tag
    RabbitMQDelivery any `json:"-"` // RabbitMQ delivery 对象（用于 Ack/Nack）
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
