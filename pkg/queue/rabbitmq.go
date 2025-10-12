package queue

import (
	"github.com/z-wentao/voiceflow/pkg/models"
)

// RabbitMQQueue RabbitMQ 队列实现（预留）
// TODO: 未来实现时需要引入 github.com/rabbitmq/amqp091-go
type RabbitMQQueue struct {
	// connection *amqp.Connection
	// channel    *amqp.Channel
	// queueName  string
}

// NewRabbitMQQueue 创建 RabbitMQ 队列
func NewRabbitMQQueue(url, queueName string) (*RabbitMQQueue, error) {
	// TODO: 实现 RabbitMQ 连接
	return nil, nil
}

// Enqueue 将任务加入队列
func (rq *RabbitMQQueue) Enqueue(job *models.TranscriptionJob) error {
	// TODO: 实现 RabbitMQ 发送
	return nil
}

// Dequeue 从队列取出任务
func (rq *RabbitMQQueue) Dequeue() (*models.TranscriptionJob, error) {
	// TODO: 实现 RabbitMQ 接收
	return nil, nil
}

// Close 关闭队列
func (rq *RabbitMQQueue) Close() error {
	// TODO: 实现关闭连接
	return nil
}
