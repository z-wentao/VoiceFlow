package storage

import "github.com/z-wentao/voiceflow/pkg/models"

// Store 任务存储接口
type Store interface {
    // Save 保存任务
    Save(job *models.TranscriptionJob) error

    // Get 获取任务
    Get(jobID string) (*models.TranscriptionJob, error)

    // Update 更新任务（使用回调函数模式）
    Update(jobID string, updateFn func(*models.TranscriptionJob)) error

    // List 列出所有任务
    List() ([]*models.TranscriptionJob, error)

    // List all jobs history
    ListAll() ([]*models.TranscriptionJob, error)

    // Delete 删除任务
    Delete(jobID string) error

    // Close 关闭存储连接
    Close() error
}
