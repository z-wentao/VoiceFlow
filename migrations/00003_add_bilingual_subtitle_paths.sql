-- +goose Up
ALTER TABLE transcription_jobs ADD COLUMN vtt_path VARCHAR(500);
ALTER TABLE transcription_jobs ADD COLUMN bilingual_srt_path VARCHAR(500);
ALTER TABLE transcription_jobs ADD COLUMN bilingual_vtt_path VARCHAR(500);

COMMENT ON COLUMN transcription_jobs.vtt_path IS 'VTT字幕文件存储路径';
COMMENT ON COLUMN transcription_jobs.bilingual_srt_path IS '双语SRT字幕文件存储路径';
COMMENT ON COLUMN transcription_jobs.bilingual_vtt_path IS '双语VTT字幕文件存储路径';

-- +goose Down
ALTER TABLE transcription_jobs DROP COLUMN vtt_path;
ALTER TABLE transcription_jobs DROP COLUMN bilingual_srt_path;
ALTER TABLE transcription_jobs DROP COLUMN bilingual_vtt_path;
