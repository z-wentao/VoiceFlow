package storage

import (
    "log"
    "time"

    "github.com/z-wentao/voiceflow/pkg/models"
)

// HybridJobStore 混合存储：Redis（热数据） + PostgreSQL（冷数据）
// 面试亮点：双层架构，平衡性能和可靠性
type HybridJobStore struct {
    redis     Store                                 // Redis 存储（快速缓存）
    db        Store                                 // PostgreSQL 存储（持久化）
    syncQueue chan *models.TranscriptionJob        // 异步同步队列
    stopCh    chan struct{}                         // 停止信号
}

// NewHybridJobStore 创建混合存储
func NewHybridJobStore(redis, db Store) *HybridJobStore {
    store := &HybridJobStore{
	redis:     redis,
	db:        db,
	syncQueue: make(chan *models.TranscriptionJob, 100),
	stopCh:    make(chan struct{}),
    }

    // 启动后台同步 Worker
    go store.syncWorker()

    log.Println("✓ 混合存储初始化成功（Redis + PostgreSQL）")

    return store
}

// Save 保存任务
// 策略：立即写 Redis，异步写数据库
func (s *HybridJobStore) Save(job *models.TranscriptionJob) error {
    // 1. 快速写入 Redis（用户立即可查询）
    if err := s.redis.Save(job); err != nil {
	log.Printf("⚠️ Redis 写入失败: %v", err)
	// Redis 失败不影响业务，继续写数据库
    }

    // 2. 异步写入数据库（仅完成或失败的任务）
    if job.Status == models.StatusCompleted || job.Status == models.StatusFailed {
	s.asyncSyncToDB(job)
    }

    return nil
}

// Get 获取任务
// 策略：优先 Redis，未命中查数据库并回写 Redis
func (s *HybridJobStore) Get(jobID string) (*models.TranscriptionJob, error) {
    // 1. 先查 Redis（缓存命中，快速返回）
    job, err := s.redis.Get(jobID)
    if err == nil {
	return job, nil
    }

    // 2. Redis 未命中，查数据库
    log.Printf("📚 Redis 缓存未命中，查询数据库: %s", jobID)
    job, err = s.db.Get(jobID)
    if err != nil {
	return nil, err
    }

    // 3. 回写 Redis（缓存预热，下次查询更快）
    go func() {
	if err := s.redis.Save(job); err != nil {
	    log.Printf("⚠️ 回写 Redis 失败: %v", err)
	}
    }()

    return job, nil
}

// Update 更新任务
// 策略：只更新 Redis（快速），完成时同步数据库
func (s *HybridJobStore) Update(jobID string, updateFn func(*models.TranscriptionJob)) error {
    // 1. 更新 Redis（快速响应）
    err := s.redis.Update(jobID, updateFn)
    if err != nil {
	log.Printf("⚠️ Redis 更新失败: %v, 尝试更新数据库", err)
	// Redis 失败，尝试更新数据库
	return s.db.Update(jobID, updateFn)
    }

    // 2. 如果任务完成或失败，同步到数据库
    job, _ := s.redis.Get(jobID)
    if job != nil && (job.Status == models.StatusCompleted || job.Status == models.StatusFailed) {
	s.asyncSyncToDB(job)
    }

    return nil
}

// List 列出任务
// 策略：优先 Redis，失败降级到数据库
func (s *HybridJobStore) List() ([]*models.TranscriptionJob, error) {
    // 优先从 Redis 获取（最近的热数据）
    jobs, err := s.redis.List()
    if err != nil {
	// Redis 失败，降级到数据库
	log.Printf("⚠️ Redis 列表查询失败: %v, 降级到数据库", err)
	return s.db.List()
    }

    return jobs, nil
}

// Delete 删除任务
// 策略：同时删除 Redis 和数据库中的数据
func (s *HybridJobStore) Delete(jobID string) error {
    // 1. 删除 Redis 中的数据
    if err := s.redis.Delete(jobID); err != nil {
	log.Printf("⚠️ Redis 删除失败: %v", err)
	// Redis 删除失败不影响整体流程
    }

    // 2. 删除数据库中的数据（确保持久化数据被清理）
    if err := s.db.Delete(jobID); err != nil {
	log.Printf("⚠️ 数据库删除失败: %v", err)
	return err
    }

    return nil
}

// Close 关闭存储
func (s *HybridJobStore) Close() error {
    // 1. 停止同步 Worker
    close(s.stopCh)

    // 2. 等待队列清空（最多等待 5 秒）
    timeout := time.After(5 * time.Second)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
	select {
	case <-timeout:
	    log.Printf("⚠️ 同步队列清空超时，剩余 %d 个任务", len(s.syncQueue))
	    goto cleanup
	case <-ticker.C:
	    if len(s.syncQueue) == 0 {
		goto cleanup
	    }
	}
    }

    cleanup:
    // 3. 关闭存储
    s.redis.Close()
    s.db.Close()

    log.Println("✓ 混合存储已关闭")
    return nil
}

// asyncSyncToDB 异步同步到数据库
func (s *HybridJobStore) asyncSyncToDB(job *models.TranscriptionJob) {
    select {
    case s.syncQueue <- job:
    // 成功加入队列
    default:
	// 队列满，同步写入（阻塞）
	log.Printf("⚠️ 同步队列已满，同步写入数据库")
	if err := s.db.Save(job); err != nil {
	    log.Printf("❌ 同步写入数据库失败: %v", err)
	}
    }
}

// syncWorker 后台同步 Worker
// 策略：批量写入（50条或5秒）
func (s *HybridJobStore) syncWorker() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    batch := make([]*models.TranscriptionJob, 0, 50)

    for {
	select {
	case job, ok := <-s.syncQueue:
	    if !ok {
		// 队列关闭，写入剩余数据
		s.batchSave(batch)
		return
	    }

	    batch = append(batch, job)

	    // 批量写入（达到 50 条）
	    if len(batch) >= 50 {
		s.batchSave(batch)
		batch = batch[:0]
	    }

	case <-ticker.C:
	    // 定时写入（5秒）
	    if len(batch) > 0 {
		s.batchSave(batch)
		batch = batch[:0]
	    }

	case <-s.stopCh:
	    // 收到停止信号
	    s.batchSave(batch)
	    return
	}
    }
}

// batchSave 批量保存到数据库
func (s *HybridJobStore) batchSave(jobs []*models.TranscriptionJob) {
    if len(jobs) == 0 {
	return
    }

    log.Printf("🔄 批量同步 %d 个任务到数据库", len(jobs))

    successCount := 0
    for _, job := range jobs {
	if err := s.db.Save(job); err != nil {
	    log.Printf("❌ 同步任务失败: %s, 错误: %v", job.JobID, err)
	} else {
	    successCount++
	}
    }

    log.Printf("✓ 成功同步 %d/%d 个任务到数据库", successCount, len(jobs))
}
