package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/z-wentao/voiceflow/pkg/config"
	"github.com/z-wentao/voiceflow/pkg/maimemo_service"
	"github.com/z-wentao/voiceflow/pkg/models"
	"github.com/z-wentao/voiceflow/pkg/queue"
	"github.com/z-wentao/voiceflow/pkg/storage"
	"github.com/z-wentao/voiceflow/pkg/transcriber"
	"github.com/z-wentao/voiceflow/pkg/vocabulary"
	"github.com/z-wentao/voiceflow/pkg/worker"
)

// App 应用上下文（面试亮点：依赖注入）
type App struct {
	config         *config.Config
	queue          queue.Queue
	store          storage.Store // 改为接口类型，支持多种存储实现
	workers        []*worker.Worker
	engine         *transcriber.TranscriptionEngine
	extractor      *vocabulary.Extractor
	maimemoService *maimemo_service.Client // Maimemo 微服务客户端
}

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}
	log.Println("✓ 配置加载成功")

	// 2. 确保必要的目录存在
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatalf("❌ 创建 uploads 目录失败: %v", err)
	}

	// 3. 初始化组件
	app := &App{
		config: cfg,
	}

	// 4. 初始化存储（根据配置选择类型）
	switch cfg.Storage.Type {
	case "memory":
		app.store = storage.NewJobStore()
		log.Println("✓ 使用内存存储")
	case "redis":
		ttl := time.Duration(cfg.Storage.Redis.TTL) * time.Hour
		app.store, err = storage.NewRedisJobStore(
			cfg.Storage.Redis.Addr,
			cfg.Storage.Redis.Password,
			cfg.Storage.Redis.DB,
			ttl,
		)
		if err != nil {
			log.Fatalf("❌ 初始化 Redis 存储失败: %v", err)
		}
		log.Printf("✓ 使用 Redis 存储 (地址: %s, TTL: %d 小时)", cfg.Storage.Redis.Addr, cfg.Storage.Redis.TTL)
	case "postgres":
		// 构建 PostgreSQL 连接字符串
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Storage.Postgres.Host,
			cfg.Storage.Postgres.Port,
			cfg.Storage.Postgres.User,
			cfg.Storage.Postgres.Password,
			cfg.Storage.Postgres.Database,
			cfg.Storage.Postgres.SSLMode,
		)
		app.store, err = storage.NewPostgresJobStore(connStr)
		if err != nil {
			log.Fatalf("❌ 初始化 PostgreSQL 存储失败: %v", err)
		}
		log.Printf("✓ 使用 PostgreSQL 存储 (数据库: %s@%s:%d/%s)",
			cfg.Storage.Postgres.User,
			cfg.Storage.Postgres.Host,
			cfg.Storage.Postgres.Port,
			cfg.Storage.Postgres.Database,
		)
	case "hybrid":
		// 初始化 Redis 存储（热数据）
		ttl := time.Duration(cfg.Storage.Redis.TTL) * time.Hour
		redisStore, err := storage.NewRedisJobStore(
			cfg.Storage.Redis.Addr,
			cfg.Storage.Redis.Password,
			cfg.Storage.Redis.DB,
			ttl,
		)
		if err != nil {
			log.Fatalf("❌ 初始化 Redis 存储失败: %v", err)
		}

		// 初始化 PostgreSQL 存储（冷数据）
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Storage.Postgres.Host,
			cfg.Storage.Postgres.Port,
			cfg.Storage.Postgres.User,
			cfg.Storage.Postgres.Password,
			cfg.Storage.Postgres.Database,
			cfg.Storage.Postgres.SSLMode,
		)
		dbStore, err := storage.NewPostgresJobStore(connStr)
		if err != nil {
			log.Fatalf("❌ 初始化 PostgreSQL 存储失败: %v", err)
		}

		// 创建混合存储
		app.store = storage.NewHybridJobStore(redisStore, dbStore)
		log.Printf("✓ 使用混合存储 (Redis: %s + PostgreSQL: %s/%s)",
			cfg.Storage.Redis.Addr,
			cfg.Storage.Postgres.Host,
			cfg.Storage.Postgres.Database,
		)
	default:
		log.Fatalf("❌ 不支持的存储类型: %s", cfg.Storage.Type)
	}

	// 6. 初始化队列（根据配置选择类型）
	switch cfg.Queue.Type {
	case "memory":
		app.queue = queue.NewMemoryQueue(cfg.Queue.BufferSize)
		log.Println("✓ 使用内存队列")
	case "rabbitmq":
		// TODO: 未来实现 RabbitMQ
		log.Println("⚠️  RabbitMQ 尚未实现，使用内存队列")
		app.queue = queue.NewMemoryQueue(cfg.Queue.BufferSize)
	default:
		log.Fatalf("❌ 不支持的队列类型: %s", cfg.Queue.Type)
	}

	// 8. 初始化转换引擎
	app.engine = transcriber.NewTranscriptionEngine(
		cfg.OpenAI.APIKey,
		cfg.Transcriber.SegmentConcurrency,
		cfg.Transcriber.SegmentDuration,
	)
	log.Println("✓ 转换引擎初始化成功")

	// 9. 初始化单词提取器
	app.extractor = vocabulary.NewExtractor(cfg.OpenAI.APIKey)
	log.Println("✓ 单词提取器初始化成功")

	// 10. 初始化 Maimemo 微服务客户端
	app.maimemoService = maimemo_service.NewClient(cfg.MaimemoService.URL)
	log.Printf("✓ Maimemo 微服务客户端初始化成功 (地址: %s)", cfg.MaimemoService.URL)

	// 11. 启动 Worker 池
	workerPoolSize := cfg.Transcriber.WorkerPoolSize
	app.workers = make([]*worker.Worker, workerPoolSize)

	log.Printf("🚀 正在启动 %d 个 Worker 实例...", workerPoolSize)
	for i := 0; i < workerPoolSize; i++ {
		app.workers[i] = worker.NewWorker(i+1, app.queue, app.store, app.engine)
		app.workers[i].Start()
	}

	// 12. 启动 HTTP 服务器
	router := app.setupRouter()
	port := fmt.Sprintf(":%d", cfg.Server.Port)

	log.Printf("🚀 VoiceFlow 服务器启动在 http://localhost:%d", cfg.Server.Port)
	log.Printf("📝 配置信息:")
	log.Printf("   - Worker 实例数: %d (同时处理 %d 个音频文件)", cfg.Transcriber.WorkerPoolSize, cfg.Transcriber.WorkerPoolSize)
	log.Printf("   - 每个音频的分片并发数: %d", cfg.Transcriber.SegmentConcurrency)
	log.Printf("   - 音频分片时长: %d 秒", cfg.Transcriber.SegmentDuration)
	log.Printf("   - 队列类型: %s", cfg.Queue.Type)
	log.Printf("   - 存储类型: %s", cfg.Storage.Type)
	log.Printf("   - Maimemo 微服务: %s", cfg.MaimemoService.URL)

	// 13. 优雅关闭（面试亮点）
	go func() {
		if err := router.Run(port); err != nil {
			log.Fatalf("❌ 服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 正在关闭服务器...")

	// 停止所有 Worker
	for i, w := range app.workers {
		log.Printf("正在停止 Worker #%d...", i+1)
		w.Stop()
	}

	// 关闭队列和存储
	app.queue.Close()
	app.store.Close()
	log.Println("✓ 服务器已关闭")
}

// setupRouter 设置路由
func (app *App) setupRouter() *gin.Engine {
	r := gin.Default()

	// 静态文件
	r.StaticFile("/", "./web/index.html")
	r.Static("/uploads", "./uploads")

	// API 路由
	api := r.Group("/api")
	{
		api.GET("/ping", app.handlePing)
		api.POST("/upload", app.handleUpload)
		api.GET("/jobs/:job_id", app.handleGetJob)                                // 获取任务状态
		api.GET("/jobs", app.handleListJobs)                                       // 列出所有任务
		api.DELETE("/jobs/:job_id", app.handleDeleteJob)                           // 删除任务
		api.POST("/jobs/:job_id/extract-vocabulary", app.handleExtractVocabulary) // 提取单词
		api.POST("/jobs/:job_id/sync-to-maimemo", app.handleSyncToMaimemo)        // 同步到墨墨
		api.POST("/maimemo/list-notepads", app.handleListNotepads)                // 查询云词本列表
	}

	return r
}

// isValidAudioFormat 验证音频文件格式
// Whisper API 支持的格式：mp3, mp4, mpeg, mpga, m4a, wav, webm, flac, aac
func isValidAudioFormat(ext string) bool {
	validFormats := map[string]bool{
		".mp3":  true,
		".mp4":  true, // 视频文件，但 Whisper 可以提取音频
		".mpeg": true,
		".mpga": true,
		".m4a":  true,
		".wav":  true,
		".webm": true,
		".flac": true,
		".aac":  true,
	}

	// 转为小写比较
	ext = strings.ToLower(ext)
	return validFormats[ext]
}

// handlePing 健康检查
func (app *App) handlePing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"version": "0.2.0",
	})
}

// handleUpload 处理文件上传
func (app *App) handleUpload(c *gin.Context) {
	// 1. 获取文件
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(400, gin.H{"error": "请上传文件"})
		return
	}

	// 2. 验证文件格式
	ext := filepath.Ext(file.Filename)
	if !isValidAudioFormat(ext) {
		c.JSON(400, gin.H{
			"error": fmt.Sprintf("不支持的文件格式 %s，支持: .mp3, .wav, .m4a, .mp4, .flac, .aac", ext),
		})
		return
	}

	// 3. 验证文件大小
	if file.Size > app.config.Server.MaxUploadSize {
		c.JSON(400, gin.H{
			"error": fmt.Sprintf("文件太大，最大 %.0f MB", float64(app.config.Server.MaxUploadSize)/1024/1024),
		})
		return
	}

	// 4. 生成唯一文件名
	jobID := uuid.New().String()
	filename := jobID + ext
	savePath := filepath.Join("uploads", filename)

	// 4. 保存文件
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(500, gin.H{"error": "保存文件失败"})
		return
	}

	log.Printf("✓ 文件已保存: %s (%.2f MB)", filename, float64(file.Size)/1024/1024)

	// 5. 创建任务
	job := &models.TranscriptionJob{
		JobID:     jobID,
		Filename:  file.Filename,
		FilePath:  savePath,
		Status:    models.StatusPending,
		Progress:  0,
		CreatedAt: time.Now(),
	}

	// 6. 保存到存储
	if err := app.store.Save(job); err != nil {
		c.JSON(500, gin.H{"error": "保存任务失败"})
		return
	}

	// 7. 加入队列（面试亮点：异步处理）
	if err := app.queue.Enqueue(job); err != nil {
		c.JSON(500, gin.H{"error": "任务加入队列失败"})
		return
	}

	log.Printf("✓ 任务已加入队列: %s", jobID)

	// 8. 返回结果
	c.JSON(200, gin.H{
		"job_id":   jobID,
		"filename": file.Filename,
		"size":     file.Size,
		"status":   job.Status,
		"message":  "上传成功，正在处理中...",
	})
}

// handleGetJob 获取任务状态
func (app *App) handleGetJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := app.store.Get(jobID)
	if err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(200, job)
}

// handleListJobs 列出所有任务
func (app *App) handleListJobs(c *gin.Context) {
	jobs, err := app.store.List()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取任务列表失败"})
		return
	}
	c.JSON(200, gin.H{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// handleDeleteJob 删除任务
func (app *App) handleDeleteJob(c *gin.Context) {
	jobID := c.Param("job_id")

	// 1. 从存储中删除任务
	if err := app.store.Delete(jobID); err != nil {
		log.Printf("❌ 删除任务失败: %v", err)
		c.JSON(404, gin.H{"error": "任务不存在或删除失败"})
		return
	}

	log.Printf("✓ 任务已删除: %s", jobID)

	// 2. 返回成功
	c.JSON(200, gin.H{
		"message": "删除成功",
		"job_id":  jobID,
	})
}

// handleExtractVocabulary 提取单词
func (app *App) handleExtractVocabulary(c *gin.Context) {
	jobID := c.Param("job_id")

	// 1. 获取任务
	job, err := app.store.Get(jobID)
	if err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	// 2. 检查任务是否已完成
	if job.Status != models.StatusCompleted {
		c.JSON(400, gin.H{"error": "任务尚未完成，无法提取单词"})
		return
	}

	// 3. 检查是否有转换结果
	if job.Result == "" {
		c.JSON(400, gin.H{"error": "转换结果为空"})
		return
	}

	// 4. 使用 AI 提取单词
	log.Printf("开始提取单词，任务 ID: %s", jobID)
	result, err := app.extractor.Extract(c.Request.Context(), job.Result)
	if err != nil {
		log.Printf("❌ 提取单词失败: %v", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("提取单词失败: %v", err)})
		return
	}

	// 5. 保存到任务
	job.Vocabulary = result.Words
	job.VocabDetail = make([]models.WordDetail, len(result.Details))
	for i, detail := range result.Details {
		job.VocabDetail[i] = models.WordDetail{
			Word:       detail.Word,
			Definition: detail.Definition,
			Example:    detail.Example,
		}
	}

	if err := app.store.Save(job); err != nil {
		c.JSON(500, gin.H{"error": "保存单词列表失败"})
		return
	}

	log.Printf("✓ 成功提取 %d 个单词", len(result.Words))

	// 6. 返回结果
	c.JSON(200, gin.H{
		"job_id":      jobID,
		"vocabulary":  job.Vocabulary,
		"vocab_detail": job.VocabDetail,
		"count":       len(job.Vocabulary),
	})
}

// SyncToMaimemoRequest 同步到墨墨的请求
type SyncToMaimemoRequest struct {
	Token     string `json:"token" binding:"required"`      // 墨墨 API Token
	NotepadID string `json:"notepad_id" binding:"required"` // 云词本 ID
}

// handleSyncToMaimemo 同步到墨墨背单词（通过 Maimemo 微服务）
func (app *App) handleSyncToMaimemo(c *gin.Context) {
	jobID := c.Param("job_id")

	// 1. 解析请求
	var req SyncToMaimemoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 2. 获取任务
	job, err := app.store.Get(jobID)
	if err != nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	// 3. 检查是否已提取单词
	if len(job.Vocabulary) == 0 {
		c.JSON(400, gin.H{"error": "尚未提取单词，请先调用提取单词接口"})
		return
	}

	// 4. 调用 Maimemo 微服务添加单词
	log.Printf("开始同步到墨墨，任务 ID: %s, 单词数: %d", jobID, len(job.Vocabulary))
	if err := app.maimemoService.AddWordsToNotepad(c.Request.Context(), req.Token, req.NotepadID, job.Vocabulary); err != nil {
		log.Printf("❌ 同步到墨墨失败: %v", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("同步到墨墨失败: %v", err)})
		return
	}

	log.Printf("✓ 成功同步 %d 个单词到墨墨", len(job.Vocabulary))

	// 5. 返回结果
	c.JSON(200, gin.H{
		"message": "同步成功",
		"count":   len(job.Vocabulary),
	})
}

// ListNotepadsRequest 查询云词本列表的请求
type ListNotepadsRequest struct {
	Token string `json:"token" binding:"required"` // 墨墨 API Token
}

// handleListNotepads 查询用户的云词本列表（通过 Maimemo 微服务）
func (app *App) handleListNotepads(c *gin.Context) {
	// 1. 解析请求
	var req ListNotepadsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 2. 调用 Maimemo 微服务获取云词本列表
	log.Printf("正在查询云词本列表...")
	notepads, err := app.maimemoService.ListNotepads(c.Request.Context(), req.Token)
	if err != nil {
		log.Printf("❌ 查询云词本列表失败: %v", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("查询失败: %v", err)})
		return
	}

	log.Printf("✓ 成功查询到 %d 个云词本", len(notepads))

	// 3. 返回结果
	c.JSON(200, gin.H{
		"notepads": notepads,
		"count":    len(notepads),
	})
}
