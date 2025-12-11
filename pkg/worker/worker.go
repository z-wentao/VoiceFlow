package worker

import (
    "context"
    "log"
    "strings"
    "time"

    "github.com/z-wentao/voiceflow/pkg/models"
    "github.com/z-wentao/voiceflow/pkg/queue"
    "github.com/z-wentao/voiceflow/pkg/storage"
    "github.com/z-wentao/voiceflow/pkg/transcriber"
)

// Worker 任务处理器
type Worker struct {
    id     int
    queue  queue.Queue
    store  storage.Store
    engine *transcriber.TranscriptionEngine
    ctx    context.Context
    cancel context.CancelFunc
}

func NewWorker(
    id int,
    q queue.Queue,
    store storage.Store,
    engine *transcriber.TranscriptionEngine,
) *Worker {
    ctx, cancel := context.WithCancel(context.Background())

    return &Worker{
	id:     id,
	queue:  q,
	store:  store,
	engine: engine,
	ctx:    ctx,
	cancel: cancel,
    }
}

// Start 启动 Worker（在独立的 Goroutine 中运行）
// 面试亮点：优雅的启动和关闭
func (w *Worker) Start() {
    go w.run()
}

// Stop 停止 Worker
func (w *Worker) Stop() {
    log.Printf("[Worker-%d] 正在停止...", w.id)
    w.cancel()
}

// run Worker 主循环
func (w *Worker) run() {
    log.Printf("[Worker-%d] 已启动，等待任务...", w.id)

    for {
	// 检查是否需要停止
	select {
	case <-w.ctx.Done():
	    log.Printf("[Worker-%d] 已停止", w.id)
	    return
	default:
	}

	// 从队列获取任务（阻塞）
	job, err := w.queue.Dequeue()
	if err != nil {
	    log.Printf("[Worker-%d] 从队列获取任务失败: %v", w.id, err)
	    time.Sleep(1 * time.Second)
	    continue
	}

	// 处理任务
	w.processJob(job)
    }
}

// processJob 处理单个任务
func (w *Worker) processJob(job *models.TranscriptionJob) {
    log.Printf("\n" + strings.Repeat("=", 80))
    log.Printf("[Worker-%d] 📝 开始处理任务: %s", w.id, job.JobID)
    log.Printf("[Worker-%d] 📂 文件名: %s", w.id, job.Filename)

    // 更新状态为处理中
    w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
	j.Status = models.StatusProcessing
	j.Progress = 0
    })

    // 进度回调
    progressCallback := func(progress int) {
	w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
	    j.Progress = progress
	})
	log.Printf("[Worker-%d] 任务 %s 进度: %d%%", w.id, job.JobID, progress)
    }

    ctx, cancel := context.WithTimeout(w.ctx, 30*time.Minute)
    defer cancel()

    // 调用转换引擎
    startTime := time.Now()
    result, err := w.engine.Transcribe(ctx, job.FilePath, "", progressCallback)

    if err != nil {
	// 处理失败
	log.Printf("[Worker-%d] ❌ 任务 %s 失败: %v", w.id, job.JobID, err)
	w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
	    j.Status = models.StatusFailed
	    j.Error = err.Error()
	    j.CompletedAt = time.Now()
	})

	// 拒绝消息（不重新入队，避免无限重试）
	// 注意：RabbitMQ 会执行真实的 Nack，MemoryQueue 则是空操作
	if nackErr := w.queue.Nack(job, false); nackErr != nil {
	    log.Printf("[Worker-%d] ⚠️  Nack 消息失败: %v", w.id, nackErr)
	}
	return
    }

    // 处理成功
    duration := time.Since(startTime)
    log.Printf("[Worker-%d] 🎉 任务 %s 完成！", w.id, job.JobID)
    log.Printf("[Worker-%d] ⏱️  总耗时: %.2f 秒 (%.2f 分钟)", w.id, duration.Seconds(), duration.Minutes())
    log.Printf("[Worker-%d] 📝 转换结果长度: %d 字符", w.id, len(result.Text))
    if result.SubtitlePath != "" {
	log.Printf("[Worker-%d] 🎬 字幕文件:", w.id)
	log.Printf("[Worker-%d]    - SRT: %s", w.id, result.SubtitlePath)
	log.Printf("[Worker-%d]    - VTT: %s", w.id, result.VTTPath)
    }
    log.Printf(strings.Repeat("=", 80) + "\n")

    w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
	j.Status = models.StatusCompleted
	j.Result = result.Text
	j.SubtitlePath = result.SubtitlePath
	j.VTTPath = result.VTTPath
	j.Progress = 100
	j.CompletedAt = time.Now()
    })

    // 确认消息（任务成功完成）
    // 注意：RabbitMQ 会执行真实的 Ack，MemoryQueue 则是空操作
    if err := w.queue.Ack(job); err != nil {
	log.Printf("[Worker-%d] ⚠️  确认消息失败: %v", w.id, err)
    }
}
