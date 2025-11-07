-- +goose Up
-- +goose StatementBegin
-- 创建任务表
CREATE TABLE IF NOT EXISTS transcription_jobs (
    job_id VARCHAR(36) PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(500),
    status VARCHAR(20) NOT NULL,
    progress INT DEFAULT 0,
    result TEXT,
    language VARCHAR(10),
    duration FLOAT,
    error TEXT,
    vocabulary JSONB,
    vocab_detail JSONB,
    created_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,

    -- 索引优化
    CONSTRAINT check_status CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT check_progress CHECK (progress >= 0 AND progress <= 100)
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_jobs_status ON transcription_jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON transcription_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_status_created ON transcription_jobs(status, created_at DESC);

-- 添加注释
COMMENT ON TABLE transcription_jobs IS '音频转录任务表';
COMMENT ON COLUMN transcription_jobs.job_id IS '任务唯一ID';
COMMENT ON COLUMN transcription_jobs.filename IS '原始文件名';
COMMENT ON COLUMN transcription_jobs.file_path IS '服务器存储路径';
COMMENT ON COLUMN transcription_jobs.status IS '任务状态：pending/processing/completed/failed';
COMMENT ON COLUMN transcription_jobs.progress IS '处理进度 0-100';
COMMENT ON COLUMN transcription_jobs.result IS '转录结果文本';
COMMENT ON COLUMN transcription_jobs.vocabulary IS '提取的单词列表（JSON数组）';
COMMENT ON COLUMN transcription_jobs.vocab_detail IS '单词详细信息（JSON数组）';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transcription_jobs;
-- +goose StatementEnd
