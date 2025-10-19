package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/z-wentao/voiceflow/pkg/models"
)

// PostgresJobStore PostgreSQL 任务存储
// 面试亮点：使用关系数据库实现持久化存储，支持复杂查询
type PostgresJobStore struct {
	db *sql.DB
}

// NewPostgresJobStore 创建 PostgreSQL 任务存储
func NewPostgresJobStore(connStr string) (*PostgresJobStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &PostgresJobStore{db: db}, nil
}

// Save 保存任务（使用 UPSERT）
func (s *PostgresJobStore) Save(job *models.TranscriptionJob) error {
	// 序列化 JSON 字段
	vocabularyJSON, err := json.Marshal(job.Vocabulary)
	if err != nil {
		return fmt.Errorf("序列化 vocabulary 失败: %w", err)
	}

	vocabDetailJSON, err := json.Marshal(job.VocabDetail)
	if err != nil {
		return fmt.Errorf("序列化 vocab_detail 失败: %w", err)
	}

	query := `
		INSERT INTO transcription_jobs (
			job_id, filename, file_path, status, progress,
			result, language, duration, error,
			vocabulary, vocab_detail, created_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (job_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			progress = EXCLUDED.progress,
			result = EXCLUDED.result,
			language = EXCLUDED.language,
			duration = EXCLUDED.duration,
			error = EXCLUDED.error,
			vocabulary = EXCLUDED.vocabulary,
			vocab_detail = EXCLUDED.vocab_detail,
			completed_at = EXCLUDED.completed_at
	`

	_, err = s.db.Exec(query,
		job.JobID,
		job.Filename,
		job.FilePath,
		job.Status,
		job.Progress,
		job.Result,
		job.Language,
		job.Duration,
		job.Error,
		vocabularyJSON,
		vocabDetailJSON,
		job.CreatedAt,
		job.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("保存到数据库失败: %w", err)
	}

	return nil
}

// Get 获取任务
func (s *PostgresJobStore) Get(jobID string) (*models.TranscriptionJob, error) {
	query := `
		SELECT job_id, filename, file_path, status, progress,
		       result, language, duration, error,
		       vocabulary, vocab_detail, created_at, completed_at
		FROM transcription_jobs
		WHERE job_id = $1
	`

	var job models.TranscriptionJob
	var vocabularyJSON, vocabDetailJSON []byte
	var result, language, errorMsg sql.NullString
	var filePath sql.NullString
	var duration sql.NullFloat64
	var completedAt sql.NullTime

	err := s.db.QueryRow(query, jobID).Scan(
		&job.JobID,
		&job.Filename,
		&filePath,
		&job.Status,
		&job.Progress,
		&result,
		&language,
		&duration,
		&errorMsg,
		&vocabularyJSON,
		&vocabDetailJSON,
		&job.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("查询数据库失败: %w", err)
	}

	// 处理 NULL 值
	if filePath.Valid {
		job.FilePath = filePath.String
	}
	if result.Valid {
		job.Result = result.String
	}
	if language.Valid {
		job.Language = language.String
	}
	if duration.Valid {
		job.Duration = duration.Float64
	}
	if errorMsg.Valid {
		job.Error = errorMsg.String
	}
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}

	// 反序列化 JSON 字段
	if len(vocabularyJSON) > 0 {
		json.Unmarshal(vocabularyJSON, &job.Vocabulary)
	}
	if len(vocabDetailJSON) > 0 {
		json.Unmarshal(vocabDetailJSON, &job.VocabDetail)
	}

	return &job, nil
}

// Update 更新任务
func (s *PostgresJobStore) Update(jobID string, updateFn func(*models.TranscriptionJob)) error {
	// 1. 获取现有任务
	job, err := s.Get(jobID)
	if err != nil {
		return err
	}

	// 2. 执行更新函数
	updateFn(job)

	// 3. 保存回数据库
	return s.Save(job)
}

// List 列出所有任务（按创建时间倒序）
func (s *PostgresJobStore) List() ([]*models.TranscriptionJob, error) {
	query := `
		SELECT job_id, filename, file_path, status, progress,
		       result, language, duration, error,
		       vocabulary, vocab_detail, created_at, completed_at
		FROM transcription_jobs
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询数据库失败: %w", err)
	}
	defer rows.Close()

	jobs := make([]*models.TranscriptionJob, 0)

	for rows.Next() {
		var job models.TranscriptionJob
		var vocabularyJSON, vocabDetailJSON []byte
		var result, language, errorMsg sql.NullString
		var filePath sql.NullString
		var duration sql.NullFloat64
		var completedAt sql.NullTime

		err := rows.Scan(
			&job.JobID,
			&job.Filename,
			&filePath,
			&job.Status,
			&job.Progress,
			&result,
			&language,
			&duration,
			&errorMsg,
			&vocabularyJSON,
			&vocabDetailJSON,
			&job.CreatedAt,
			&completedAt,
		)

		if err != nil {
			continue
		}

		// 处理 NULL 值
		if filePath.Valid {
			job.FilePath = filePath.String
		}
		if result.Valid {
			job.Result = result.String
		}
		if language.Valid {
			job.Language = language.String
		}
		if duration.Valid {
			job.Duration = duration.Float64
		}
		if errorMsg.Valid {
			job.Error = errorMsg.String
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.Time
		}

		// 反序列化 JSON 字段
		if len(vocabularyJSON) > 0 {
			json.Unmarshal(vocabularyJSON, &job.Vocabulary)
		}
		if len(vocabDetailJSON) > 0 {
			json.Unmarshal(vocabDetailJSON, &job.VocabDetail)
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// Delete 删除任务
func (s *PostgresJobStore) Delete(jobID string) error {
	query := `DELETE FROM transcription_jobs WHERE job_id = $1`

	result, err := s.db.Exec(query, jobID)
	if err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除结果失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	return nil
}

// Close 关闭数据库连接
func (s *PostgresJobStore) Close() error {
	return s.db.Close()
}

// RunMigrations 运行数据库迁移
func (s *PostgresJobStore) RunMigrations(migrationPath string) error {
	// 读取迁移文件
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}

	// 执行迁移
	_, err = s.db.Exec(string(migrationSQL))
	if err != nil {
		return fmt.Errorf("执行迁移失败: %w", err)
	}

	return nil
}
