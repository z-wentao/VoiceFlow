-- +goose Up
ALTER TABLE transcription_jobs ADD COLUMN IF NOT EXISTS vtt_path VARCHAR(500);
COMMENT ON COLUMN transcription_jobs.vtt_path IS 'VTT字幕文件存储路径';

-- +goose Down
-- 不做任何操作，因为这是修复迁移
