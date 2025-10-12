package queue

import (
	"fmt"
	"github.com/z-wentao/voiceflow/pkg/models"
)

// MemoryQueue 基于 Channel 的内存队列实现
// 面试亮点：展示 Go Channel 的使用
type MemoryQueue struct {
	queue chan *models.TranscriptionJob
}

// NewMemoryQueue 创建内存队列
func NewMemoryQueue(bufferSize int) *MemoryQueue {
	return &MemoryQueue{
		queue: make(chan *models.TranscriptionJob, bufferSize),
	}
}

// Enqueue 将任务加入队列
func (mq *MemoryQueue) Enqueue(job *models.TranscriptionJob) error {
	select {
	case mq.queue <- job:
		return nil
	default:
		return fmt.Errorf("队列已满")
	}
}

// Dequeue 从队列取出任务（阻塞等待）
// 面试亮点：使用 Channel 实现阻塞等待
func (mq *MemoryQueue) Dequeue() (*models.TranscriptionJob, error) {
	job, ok := <-mq.queue
	if !ok {
		return nil, fmt.Errorf("队列已关闭")
	}
	return job, nil
}

// Close 关闭队列
func (mq *MemoryQueue) Close() error {
	close(mq.queue)
	return nil
}
