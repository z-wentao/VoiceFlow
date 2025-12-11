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

// RabbitMQQueue RabbitMQ 队列实现（简化版）
// 核心改进：
// 1. 单一 Consumer（所有 Worker 共享）
// 2. 通过 QoS prefetchCount 控制并发
// 3. 手动 Ack/Nack 保证消息可靠性
type RabbitMQQueue struct {
	url       string
	queueName string
	closed    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc

	// 发布消息用的连接和通道
	publishConn           *amqp.Connection
	publishRabbitChannel  *amqp.Channel
	publishMutex          sync.Mutex

	// 消费消息用的连接和通道
	consumeConn           *amqp.Connection
	consumeRabbitChannel  *amqp.Channel
	deliveriesGoChannel   <-chan amqp.Delivery // 所有 Worker 共享这个 Go Channel

	// 用于保护 Ack/Nack 操作（RabbitMQ Channel 不是并发安全的）
	ackMutex              sync.Mutex
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
	}

	// 1. 建立发布连接
	if err := rq.setupPublisher(); err != nil {
		cancel()
		return nil, fmt.Errorf("初始化发布者失败: %w", err)
	}

	// 2. 建立消费连接
	if err := rq.setupConsumer(); err != nil {
		cancel()
		rq.closePublisher()
		return nil, fmt.Errorf("初始化消费者失败: %w", err)
	}

	log.Printf("✓ RabbitMQ 队列初始化成功 (队列: %s)", queueName)

	return rq, nil
}

// setupPublisher 设置发布者连接（用于发送消息）
func (rq *RabbitMQQueue) setupPublisher() error {
	conn, err := amqp.Dial(rq.url)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建 RabbitMQ Channel 失败: %w", err)
	}

	// 声明持久化队列（幂等操作）
	_, err = ch.QueueDeclare(
		rq.queueName, // name
		true,         // durable: 持久化队列
		false,        // autoDelete: 不自动删除
		false,        // exclusive: 非独占
		false,        // noWait
		nil,          // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("声明队列失败: %w", err)
	}

	rq.publishConn = conn
	rq.publishRabbitChannel = ch

	log.Println("✓ RabbitMQ 发布者连接已建立")
	return nil
}

// setupConsumer 设置消费者连接（用于接收消息）
func (rq *RabbitMQQueue) setupConsumer() error {
	conn, err := amqp.Dial(rq.url)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建 RabbitMQ Channel 失败: %w", err)
	}

	// 设置 QoS：预取数量 = Worker 数量
	// 这样 RabbitMQ 会一次性推送 3 条消息到 deliveriesGoChannel
	// 3 个 Worker 各拿一条，实现并发处理
	workerCount := 3 // 可以作为参数传入，这里硬编码为 3
	err = ch.Qos(
		workerCount, // prefetchCount: 预取消息数量
		0,           // prefetchSize: 0 表示不限制
		false,       // global: false 表示只应用于当前 channel
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("设置 QoS 失败: %w", err)
	}

	// 启动消费（订阅队列）
	// 这个调用会返回一个 Go Channel，RabbitMQ 会持续往这个 channel 推送消息
	deliveries, err := ch.Consume(
		rq.queueName,  // queue: 队列名
		"consumer-1",  // consumer: consumer tag（标识符）
		false,         // autoAck: false 表示手动确认
		false,         // exclusive: 非独占
		false,         // noLocal
		false,         // noWait
		nil,           // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("启动消费失败: %w", err)
	}

	rq.consumeConn = conn
	rq.consumeRabbitChannel = ch
	rq.deliveriesGoChannel = deliveries

	log.Printf("✓ RabbitMQ 消费者已启动 (prefetchCount=%d)", workerCount)
	return nil
}

// Enqueue 将任务加入队列
func (rq *RabbitMQQueue) Enqueue(job *models.TranscriptionJob) error {
	rq.publishMutex.Lock()
	defer rq.publishMutex.Unlock()

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	// 创建上下文（5 秒超时）
	ctx, cancel := context.WithTimeout(rq.ctx, 5*time.Second)
	defer cancel()

	// 发布消息到队列
	err = rq.publishRabbitChannel.PublishWithContext(
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
// 所有 Worker goroutine 共享同一个 deliveriesGoChannel
// Go Channel 保证每条消息只会被一个 Worker 读取
func (rq *RabbitMQQueue) Dequeue() (*models.TranscriptionJob, error) {
	// 从 Go Channel 读取消息
	select {
	case <-rq.closed:
		return nil, fmt.Errorf("队列已关闭")
	case <-rq.ctx.Done():
		return nil, fmt.Errorf("队列已关闭")
	case delivery, ok := <-rq.deliveriesGoChannel:
		if !ok {
			// Go Channel 已关闭
			return nil, fmt.Errorf("消费通道已关闭")
		}

		// 反序列化任务
		var job models.TranscriptionJob
		if err := json.Unmarshal(delivery.Body, &job); err != nil {
			// 反序列化失败，拒绝消息（不重新入队）
			rq.nackInternal(delivery.DeliveryTag, false)
			return nil, fmt.Errorf("反序列化任务失败: %w", err)
		}

		// 保存 delivery 信息用于后续确认
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
	return rq.ackInternal(delivery.DeliveryTag)
}

// Nack 拒绝消息（任务处理失败）
func (rq *RabbitMQQueue) Nack(job *models.TranscriptionJob, requeue bool) error {
	if job.RabbitMQDelivery == nil {
		return nil // 不是 RabbitMQ 消息，忽略
	}

	delivery := job.RabbitMQDelivery.(*amqp.Delivery)
	return rq.nackInternal(delivery.DeliveryTag, requeue)
}

// ackInternal 内部 Ack 实现（带锁保护）
// 因为 RabbitMQ Channel 不是并发安全的，多个 Worker 可能同时调用
func (rq *RabbitMQQueue) ackInternal(deliveryTag uint64) error {
	rq.ackMutex.Lock()
	defer rq.ackMutex.Unlock()

	return rq.consumeRabbitChannel.Ack(deliveryTag, false)
}

// nackInternal 内部 Nack 实现（带锁保护）
func (rq *RabbitMQQueue) nackInternal(deliveryTag uint64, requeue bool) error {
	rq.ackMutex.Lock()
	defer rq.ackMutex.Unlock()

	return rq.consumeRabbitChannel.Nack(deliveryTag, false, requeue)
}

// Close 关闭队列
func (rq *RabbitMQQueue) Close() error {
	select {
	case <-rq.closed:
		return nil // 已经关闭
	default:
		close(rq.closed)
		rq.cancel()

		// 关闭消费连接
		if rq.consumeRabbitChannel != nil {
			rq.consumeRabbitChannel.Close()
		}
		if rq.consumeConn != nil {
			rq.consumeConn.Close()
		}

		// 关闭发布连接
		rq.closePublisher()

		log.Println("✓ RabbitMQ 队列已关闭")
		return nil
	}
}

// closePublisher 关闭发布者连接
func (rq *RabbitMQQueue) closePublisher() {
	if rq.publishRabbitChannel != nil {
		rq.publishRabbitChannel.Close()
	}
	if rq.publishConn != nil {
		rq.publishConn.Close()
	}
}

// GetQueueInfo 获取队列信息（调试用）
func (rq *RabbitMQQueue) GetQueueInfo() (messages, consumers int, err error) {
	q, err := rq.publishRabbitChannel.QueueInspect(rq.queueName)
	if err != nil {
		return 0, 0, err
	}
	return q.Messages, q.Consumers, nil
}
