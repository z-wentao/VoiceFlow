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
// 面试亮点：展示 Goroutine 的生命周期管理
type Worker struct {
	queue  queue.Queue
	store  *storage.JobStore
	engine *transcriber.TranscriptionEngine
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWorker 创建 Worker
func NewWorker(
	q queue.Queue,
	store *storage.JobStore,
	engine *transcriber.TranscriptionEngine,
) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
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
	log.Println("正在停止 Worker...")
	w.cancel()
}

// run Worker 主循环
func (w *Worker) run() {
	log.Println("Worker 已启动，等待任务...")

	for {
		// 检查是否需要停止
		select {
		case <-w.ctx.Done():
			log.Println("Worker 已停止")
			return
		default:
		}

		// 从队列获取任务（阻塞）
		job, err := w.queue.Dequeue()
		if err != nil {
			log.Printf("从队列获取任务失败: %v", err)
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
	log.Printf("📝 开始处理任务: %s", job.JobID)
	log.Printf("📂 文件名: %s", job.Filename)

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
		log.Printf("任务 %s 进度: %d%%", job.JobID, progress)
	}

	// 创建任务特定的 Context（30 分钟超时）
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Minute)
	defer cancel()

	// 调用转换引擎
	startTime := time.Now()
	result, err := w.engine.Transcribe(ctx, job.FilePath, "", progressCallback)

	if err != nil {
		// 处理失败
		log.Printf("❌ 任务 %s 失败: %v", job.JobID, err)
		w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
			j.Status = models.StatusFailed
			j.Error = err.Error()
			j.CompletedAt = time.Now()
		})
		return
	}

	// 处理成功
	duration := time.Since(startTime)
	log.Printf("🎉 任务 %s 完成！", job.JobID)
	log.Printf("⏱️  总耗时: %.2f 秒 (%.2f 分钟)", duration.Seconds(), duration.Minutes())
	log.Printf("📝 转换结果长度: %d 字符", len(result))
	log.Printf(strings.Repeat("=", 80) + "\n")

	w.store.Update(job.JobID, func(j *models.TranscriptionJob) {
		j.Status = models.StatusCompleted
		j.Result = result
		j.Progress = 100
		j.CompletedAt = time.Now()
	})
}
