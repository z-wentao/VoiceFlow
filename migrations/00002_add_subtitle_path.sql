-- +goose Up
-- +goose StatementBegin
-- 添加字幕文件路径字段
ALTER TABLE transcription_jobs
ADD COLUMN subtitle_path VARCHAR(500);

COMMENT ON COLUMN transcription_jobs.subtitle_path IS 'SRT字幕文件存储路径';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transcription_jobs
DROP COLUMN subtitle_path;
-- +goose StatementEnd
