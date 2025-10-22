package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/z-wentao/voiceflow/pkg/models"
)

// RabbitMQQueue RabbitMQ 队列实现
// 面试亮点：
// 1. 消息持久化（durable queue + persistent messages）
// 2. 消息确认机制（manual ack）
// 3. 每个 Worker 独立的 consumer（真正的并发）
// 4. 连接池管理
// 5. 优雅关闭
type RabbitMQQueue struct {
	url       string
	queueName string
	closed    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc

	// 连接池（用于发布消息）
	publishConn    *amqp.Connection
	publishChannel *amqp.Channel
	publishMutex   sync.Mutex

	// Consumer 管理
	consumers []*RabbitMQConsumer
	mu        sync.Mutex
}

// RabbitMQConsumer 单个消费者（每个 Worker 一个）
type RabbitMQConsumer struct {
	id         int
	connection *amqp.Connection
	channel    *amqp.Channel
	deliveries <-chan amqp.Delivery
}

// NewRabbitMQQueue 创建 RabbitMQ 队列
func NewRabbitMQQueue(url, queueName string) (*RabbitMQQueue, error) {
	ctx, cancel := context.WithCancel(context.Background())

	rq := &RabbitMQQueue{
		url:       url,
		queueName: queueName,
		closed:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
		consumers: make([]*RabbitMQConsumer, 0),
	}

	// 建立发布连接
	if err := rq.setupPublisher(); err != nil {
		cancel()
		return nil, fmt.Errorf("初始化发布者失败: %w", err)
	}

	// 声明队列（幂等操作）
	if err := rq.declareQueue(); err != nil {
		cancel()
		rq.publishConn.Close()
		return nil, fmt.Errorf("声明队列失败: %w", err)
	}

	log.Printf("✓ RabbitMQ 队列初始化成功 (队列: %s)", queueName)

	return rq, nil
}

// setupPublisher 设置发布者连接
func (rq *RabbitMQQueue) setupPublisher() error {
	conn, err := amqp.Dial(rq.url)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建 channel 失败: %w", err)
	}

	rq.publishConn = conn
	rq.publishChannel = ch

	return nil
}

// declareQueue 声明队列（幂等操作）
func (rq *RabbitMQQueue) declareQueue() error {
	// 声明持久化队列
	_, err := rq.publishChannel.QueueDeclare(
		rq.queueName, // name
		true,         // durable: 持久化队列
		false,        // autoDelete: 不自动删除
		false,        // exclusive: 非独占
		false,        // noWait
		nil,          // args
	)
	return err
}

// createConsumer 创建一个新的消费者（每个 Worker 调用一次）
// 面试亮点：每个 Worker 有独立的 consumer，实现真正的并发
func (rq *RabbitMQQueue) createConsumer() (*RabbitMQConsumer, error) {
	rq.mu.Lock()
	consumerID := len(rq.consumers) + 1
	rq.mu.Unlock()

	// 为这个 consumer 创建独立的连接
	conn, err := amqp.Dial(rq.url)
	if err != nil {
		return nil, fmt.Errorf("consumer-%d 连接失败: %w", consumerID, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("consumer-%d 创建 channel 失败: %w", consumerID, err)
	}

	// 设置 QoS（每个 consumer 最多预取 1 条消息）
	// 这样 3 个 consumer 可以同时预取 3 条消息
	err = ch.Qos(
		1,     // prefetchCount: 每个 consumer 最多预取 1 条消息
		0,     // prefetchSize
		false, // global: false 表示只应用于当前 consumer
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("consumer-%d 设置 QoS 失败: %w", consumerID, err)
	}

	// 启动消费
	deliveries, err := ch.Consume(
		rq.queueName,                      // queue
		fmt.Sprintf("consumer-%d", consumerID), // consumer tag
		false, // autoAck: false 表示手动确认
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("consumer-%d 启动消费失败: %w", consumerID, err)
	}

	consumer := &RabbitMQConsumer{
		id:         consumerID,
		connection: conn,
		channel:    ch,
		deliveries: deliveries,
	}

	// 记录 consumer
	rq.mu.Lock()
	rq.consumers = append(rq.consumers, consumer)
	rq.mu.Unlock()

	log.Printf("✓ RabbitMQ Consumer-%d 已启动", consumerID)

	return consumer, nil
}

// Enqueue 将任务加入队列
func (rq *RabbitMQQueue) Enqueue(job *models.TranscriptionJob) error {
	rq.publishMutex.Lock()
	defer rq.publishMutex.Unlock()

	// 序列化任务
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	// 创建上下文（5 秒超时）
	ctx, cancel := context.WithTimeout(rq.ctx, 5*time.Second)
	defer cancel()

	// 发布消息
	err = rq.publishChannel.PublishWithContext(
		ctx,
		"",           // exchange: 空字符串表示默认 exchange
		rq.queueName, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent, // 消息持久化
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	return nil
}

// Dequeue 从队列取出任务（阻塞）
// 面试亮点：每次调用创建独立的 consumer，实现真正的并发
func (rq *RabbitMQQueue) Dequeue() (*models.TranscriptionJob, error) {
	// 为这个 Worker 创建独立的 consumer
	consumer, err := rq.createConsumer()
	if err != nil {
		return nil, fmt.Errorf("创建 consumer 失败: %w", err)
	}

	// 从 consumer 的 deliveries channel 读取
	select {
	case <-rq.closed:
		return nil, fmt.Errorf("队列已关闭")
	case <-rq.ctx.Done():
		return nil, fmt.Errorf("队列已关闭")
	case delivery, ok := <-consumer.deliveries:
		if !ok {
			return nil, fmt.Errorf("consumer 通道已关闭")
		}

		// 反序列化任务
		var job models.TranscriptionJob
		if err := json.Unmarshal(delivery.Body, &job); err != nil {
			// 反序列化失败，拒绝消息（不重新入队）
			delivery.Nack(false, false)
			return nil, fmt.Errorf("反序列化任务失败: %w", err)
		}

		// 保存 delivery 用于后续确认
		job.DeliveryTag = delivery.DeliveryTag
		job.RabbitMQDelivery = &delivery

		return &job, nil
	}
}

// Ack 确认消息（任务处理成功）
func (rq *RabbitMQQueue) Ack(job *models.TranscriptionJob) error {
	if job.RabbitMQDelivery == nil {
		return nil // 不是 RabbitMQ 消息，忽略
	}

	delivery := job.RabbitMQDelivery.(*amqp.Delivery)
	return delivery.Ack(false)
}

// Nack 拒绝消息（任务处理失败）
func (rq *RabbitMQQueue) Nack(job *models.TranscriptionJob, requeue bool) error {
	if job.RabbitMQDelivery == nil {
		return nil // 不是 RabbitMQ 消息，忽略
	}

	delivery := job.RabbitMQDelivery.(*amqp.Delivery)
	return delivery.Nack(false, requeue)
}

// Close 关闭队列
func (rq *RabbitMQQueue) Close() error {
	select {
	case <-rq.closed:
		return nil // 已经关闭
	default:
		close(rq.closed)
		rq.cancel()

		// 关闭所有 consumer
		rq.mu.Lock()
		for _, consumer := range rq.consumers {
			if consumer.channel != nil {
				consumer.channel.Close()
			}
			if consumer.connection != nil {
				consumer.connection.Close()
			}
		}
		rq.mu.Unlock()

		// 关闭发布连接
		if rq.publishChannel != nil {
			rq.publishChannel.Close()
		}
		if rq.publishConn != nil {
			rq.publishConn.Close()
		}

		log.Println("✓ RabbitMQ 队列已关闭")
		return nil
	}
}

// GetQueueInfo 获取队列信息（调试用）
func (rq *RabbitMQQueue) GetQueueInfo() (messages, consumers int, err error) {
	q, err := rq.publishChannel.QueueInspect(rq.queueName)
	if err != nil {
		return 0, 0, err
	}
	return q.Messages, q.Consumers, nil
}
