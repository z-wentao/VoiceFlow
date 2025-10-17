package storage

import (
	"fmt"
	"sync"

	"github.com/z-wentao/voiceflow/pkg/models"
)

// JobStore 任务存储（内存实现）
// 面试亮点：使用 RWMutex 保证并发安全
type JobStore struct {
	jobs map[string]*models.TranscriptionJob
	mu   sync.RWMutex // 读写锁
}

// NewJobStore 创建任务存储
func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*models.TranscriptionJob),
	}
}

// Save 保存任务
func (js *JobStore) Save(job *models.TranscriptionJob) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.jobs[job.JobID] = job
	return nil
}

// Get 获取任务
func (js *JobStore) Get(jobID string) (*models.TranscriptionJob, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()

	job, exists := js.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}

	return job, nil
}

// Update 更新任务状态
func (js *JobStore) Update(jobID string, updateFn func(*models.TranscriptionJob)) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.jobs[jobID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	updateFn(job)
	return nil
}

// List 列出所有任务
func (js *JobStore) List() ([]*models.TranscriptionJob, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()

	jobs := make([]*models.TranscriptionJob, 0, len(js.jobs))
	for _, job := range js.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Close 关闭存储（内存存储无需关闭）
func (js *JobStore) Close() error {
	return nil
}
