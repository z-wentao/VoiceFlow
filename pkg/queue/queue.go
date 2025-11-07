package queue

import "github.com/z-wentao/voiceflow/pkg/models"

// Queue 任务队列接口
// 面试亮点：使用接口抽象，方便后续切换到 RabbitMQ
type Queue interface {
    // Enqueue 将任务加入队列
    Enqueue(job *models.TranscriptionJob) error

    // Dequeue 从队列取出任务（阻塞）
    Dequeue() (*models.TranscriptionJob, error)

    // Ack 确认消息（任务处理成功）
    Ack(job *models.TranscriptionJob) error

    // Nack 拒绝消息（任务处理失败）
    // requeue: 是否重新入队
    Nack(job *models.TranscriptionJob, requeue bool) error

    // Close 关闭队列
    Close() error
}
