package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "sort"
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
    "github.com/z-wentao/voiceflow/pkg/templates"
    "github.com/z-wentao/voiceflow/pkg/transcriber"
    "github.com/z-wentao/voiceflow/pkg/vocabulary"
    "github.com/z-wentao/voiceflow/pkg/worker"
)

// App 应用上下文（面试亮点：依赖注入）
type App struct {
    config         *config.Config
    queue          queue.Queue
    store          storage.Store
    workers        []*worker.Worker
    engine         *transcriber.TranscriptionEngine
    extractor      *vocabulary.Extractor
    maimemoService *maimemo_service.Client // Maimemo 微服务客户端
}

func main() {
    cfg, err := config.LoadConfig("config/config.yaml")
    if err != nil {
	log.Fatalf("❌ 加载配置失败: %v", err)
    }
    log.Println("✓ 配置加载成功")

    if err := os.MkdirAll("uploads", 0755); err != nil {
	log.Fatalf("❌ 创建 uploads 目录失败: %v", err)
    }

    app := &App{
	config: cfg,
    }

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
	app.queue, err = queue.NewRabbitMQQueue(
	    cfg.Queue.RabbitMQ.URL,
	    cfg.Queue.RabbitMQ.QueueName,
	    )
	if err != nil {
	    log.Fatalf("❌ 初始化 RabbitMQ 队列失败: %v", err)
	}
	log.Printf("✓ 使用 RabbitMQ 队列 (队列名: %s)", cfg.Queue.RabbitMQ.QueueName)
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

    // 创建 HTTP 服务器实例，支持优雅关闭
    srv := &http.Server{
	Addr:    port,
	Handler: router,
    }

    log.Printf("🚀 VoiceFlow 服务器启动在 http://localhost:%d", cfg.Server.Port)
    log.Printf("📝 配置信息:")
    log.Printf("   - Worker 实例数: %d (同时处理 %d 个音频文件)", cfg.Transcriber.WorkerPoolSize, cfg.Transcriber.WorkerPoolSize)
    log.Printf("   - 每个音频的分片并发数: %d", cfg.Transcriber.SegmentConcurrency)
    log.Printf("   - 音频分片时长: %d 秒", cfg.Transcriber.SegmentDuration)
    log.Printf("   - 队列类型: %s", cfg.Queue.Type)
    log.Printf("   - 存储类型: %s", cfg.Storage.Type)
    log.Printf("   - Maimemo 微服务: %s", cfg.MaimemoService.URL)

    // 13. 优雅关闭（面试亮点）
    // 在 goroutine 中启动服务器
    go func() {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	    log.Fatalf("❌ 服务器启动失败: %v", err)
	}
    }()

    // 等待中断信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("🛑 收到关闭信号，开始优雅关闭...")

    // 1. 首先停止接受新的 HTTP 请求，并等待现有请求完成
    // 设置 30 秒的关闭超时
    shutdownTimeout := 30 * time.Second
    ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
    defer cancel()

    log.Println("📍 停止接受新的 HTTP 请求...")
    if err := srv.Shutdown(ctx); err != nil {
	log.Printf("⚠️  HTTP 服务器强制关闭: %v", err)
    } else {
	log.Println("✓ HTTP 服务器已优雅关闭（所有请求已处理完成）")
    }

    // 2. 停止所有 Worker（不再处理新的队列任务）
    log.Println("📍 停止 Worker 池...")
    for i, w := range app.workers {
	log.Printf("   正在停止 Worker #%d...", i+1)
	w.Stop()
    }
    log.Println("✓ 所有 Worker 已停止")

    // 3. 关闭队列和存储
    log.Println("📍 关闭队列和存储...")
    app.queue.Close()
    app.store.Close()

    log.Println("✅ VoiceFlow 服务器已完全关闭")
}


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

	// HTMX 路由（返回 HTML 片段）
	api.POST("/upload", app.handleUpload)
	api.GET("/jobs", app.handleListJobs)
	api.GET("/jobs/history", app.handleListJobsHistory)
	api.GET("/jobs/count", app.handleJobsCount)
	api.GET("/jobs/:job_id", app.handleGetJob)
	api.GET("/jobs/:job_id/details", app.handleJobDetails)
	api.GET("/jobs/:job_id/download", app.handleDownloadResult)
	api.GET("/jobs/:job_id/download-subtitle", app.handleDownloadSubtitle)
	api.GET("/jobs/:job_id/subtitle.vtt", app.handleSubtitleVTT)
	api.DELETE("/jobs/:job_id", app.handleDeleteJob)
	api.POST("/jobs/:job_id/extract-vocabulary", app.handleExtractVocabulary)
	api.POST("/jobs/:job_id/sync-to-maimemo", app.handleSyncToMaimemo)
	api.POST("/maimemo/list-notepads", app.handleListNotepads)
    }

    return r
}

func (app *App) handlePing(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
	"message": "pong",
	"version": "0.3.0-htmx",
    })
}

// handleUpload 处理文件上传（返回 HTML）
func (app *App) handleUpload(c *gin.Context) {
    file, err := c.FormFile("audio")
    if err != nil {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 请上传文件
	    </div>
	    `))
	return
    }

    ext := filepath.Ext(file.Filename)
    if !isValidAudioFormat(ext) {
	c.Data(http.StatusBadRequest, "text/html", []byte(fmt.Sprintf(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 不支持的文件格式 %s
	    </div>
	    `, ext)))
	return
    }

    if file.Size > app.config.Server.MaxUploadSize {
	c.Data(http.StatusBadRequest, "text/html", []byte(fmt.Sprintf(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 文件太大，最大 %.0f MB
	    </div>
	    `, float64(app.config.Server.MaxUploadSize)/1024/1024)))
	return
    }

    jobID := uuid.New().String()
    filename := jobID + ext
    savePath := filepath.Join("uploads", filename)

    if err := c.SaveUploadedFile(file, savePath); err != nil {
	c.Data(http.StatusInternalServerError, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 保存文件失败
	    </div>
	    `))
	return
    }

    log.Printf("✓ 文件已保存: %s (%.2f MB)", filename, float64(file.Size)/1024/1024)

    job := &models.TranscriptionJob{
	JobID:     jobID,
	Filename:  file.Filename,
	FilePath:  savePath,
	Status:    models.StatusPending,
	Progress:  0,
	CreatedAt: time.Now(),
    }

    if err := app.store.Save(job); err != nil {
	c.Data(http.StatusInternalServerError, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 保存任务失败
	    </div>
	    `))
	return
    }

    if err := app.queue.Enqueue(job); err != nil {
	c.Data(http.StatusInternalServerError, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 任务加入队列失败
	    </div>
	    `))
	return
    }

    log.Printf("✓ 任务已加入队列: %s", jobID)

    // 返回任务卡片 HTML
    html := templates.RenderTaskCard(job)
    c.Data(http.StatusOK, "text/html", []byte(html))
}

// handleListJobs 列出所有任务（返回 HTML）
func (app *App) handleListJobs(c *gin.Context) {
    jobs, err := app.store.List()
    if err != nil {
	c.Data(http.StatusInternalServerError, "text/html", []byte(`
	    <div class="text-center py-16 text-red-400">
	    <p class="text-5xl mb-3">❌</p>
	    <p class="text-lg">获取任务列表失败</p>
	    </div>
	    `))
	return
    }

    // 按创建时间倒序排序
    sort.Slice(jobs, func(i, j int) bool {
	return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
    })

    html := templates.RenderTasksList(jobs)
    c.Data(http.StatusOK, "text/html", []byte(html))
}

func (app *App) handleListJobsHistory(c *gin.Context) {
    jobs, err := app.store.ListAll()
    if err != nil {
	c.Data(http.StatusInternalServerError, "text/html", []byte(`
	    <div class="text-center py-16 text-red-400">
	    <p class="text-5xl mb-3">❌</p>
	    <p class="text-lg">获取任务历史失败</p>
	    </div>
	    `))
	return
    }
    // 按创建时间倒序排序
    sort.Slice(jobs, func(i, j int) bool {
	return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
    })

    html := templates.RenderTasksList(jobs)
    c.Data(http.StatusOK, "text/html", []byte(html))

}

// handleJobsCount 返回任务计数（返回 HTML）
func (app *App) handleJobsCount(c *gin.Context) {
    jobs, err := app.store.List()
    if err != nil {
	c.Data(http.StatusOK, "text/html", []byte("0 个任务"))
	return
    }

    html := fmt.Sprintf("%d 个任务", len(jobs))
    c.Data(http.StatusOK, "text/html", []byte(html))
}

// handleGetJob 获取任务状态（返回 HTML）
func (app *App) handleGetJob(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.Data(http.StatusNotFound, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 任务不存在
	    </div>
	    `))
	return
    }

    html := templates.RenderTaskCard(job)
    c.Data(http.StatusOK, "text/html", []byte(html))
}

// handleJobDetails 获取任务详情（返回 HTML）
func (app *App) handleJobDetails(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.Data(http.StatusNotFound, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 任务不存在
	    </div>
	    `))
	return
    }

    html := templates.RenderTaskDetails(job)
    c.Data(http.StatusOK, "text/html", []byte(html))
}

// handleDownloadResult 下载转录结果
func (app *App) handleDownloadResult(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
	return
    }

    if job.Status != models.StatusCompleted || job.Result == "" {
	c.JSON(http.StatusBadRequest, gin.H{"error": "任务尚未完成或无结果"})
	return
    }

    // 设置下载响应头
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_转录.txt", job.Filename))
    c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(job.Result))
}

// handleDownloadSubtitle 下载 SRT 字幕文件
func (app *App) handleDownloadSubtitle(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
	return
    }

    if job.Status != models.StatusCompleted || job.SubtitlePath == "" {
	c.JSON(http.StatusBadRequest, gin.H{"error": "任务尚未完成或无字幕文件"})
	return
    }

    // 读取 SRT 文件内容
    srtContent, err := os.ReadFile(job.SubtitlePath)
    if err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "读取字幕文件失败"})
	return
    }

    // 安全的文件名（移除特殊字符）
    safeFilename := strings.TrimSuffix(job.Filename, filepath.Ext(job.Filename))
    safeFilename = strings.ReplaceAll(safeFilename, `"`, "")

    // 设置下载响应头（修复 Safari 兼容性）
    c.Header("Content-Type", "text/plain; charset=utf-8")
    c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.srt"`, safeFilename))
    c.Header("Content-Length", fmt.Sprintf("%d", len(srtContent)))
    c.Data(http.StatusOK, "text/plain; charset=utf-8", srtContent)
}

// handleSubtitleVTT 返回 WebVTT 字幕文件（用于视频播放器）
func (app *App) handleSubtitleVTT(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
	return
    }

    if job.Status != models.StatusCompleted || job.VTTPath == "" {
	c.JSON(http.StatusBadRequest, gin.H{"error": "任务尚未完成或无字幕文件"})
	return
    }

    // 读取 VTT 文件内容
    vttContent, err := os.ReadFile(job.VTTPath)
    if err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "读取字幕文件失败"})
	return
    }

    // 设置 CORS 和响应头（允许视频播放器访问）
    c.Header("Access-Control-Allow-Origin", "*")
    c.Header("Content-Type", "text/vtt; charset=utf-8")
    c.Header("Cache-Control", "public, max-age=3600")
    c.Data(http.StatusOK, "text/vtt; charset=utf-8", vttContent)
}

// handleDeleteJob 删除任务（返回空内容，让 htmx 删除元素）
func (app *App) handleDeleteJob(c *gin.Context) {
    jobID := c.Param("job_id")

    if err := app.store.Delete(jobID); err != nil {
	log.Printf("❌ 删除任务失败: %v", err)
	c.Data(http.StatusNotFound, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 删除失败
	    </div>
	    `))
	return
    }

    log.Printf("✓ 任务已删除: %s", jobID)

    // 返回空内容，htmx 会删除目标元素
    c.Data(http.StatusOK, "text/html", []byte(""))
}

// handleExtractVocabulary 提取单词（返回 HTML）
func (app *App) handleExtractVocabulary(c *gin.Context) {
    jobID := c.Param("job_id")

    job, err := app.store.Get(jobID)
    if err != nil {
	c.Data(http.StatusNotFound, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 任务不存在
	    </div>
	    `))
	return
    }

    if job.Status != models.StatusCompleted {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="bg-yellow-50 text-yellow-800 p-3 rounded-lg text-sm">
	    ⚠️ 任务尚未完成，无法提取单词
	    </div>
	    `))
	return
    }

    if job.Result == "" {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="bg-yellow-50 text-yellow-800 p-3 rounded-lg text-sm">
	    ⚠️ 转换结果为空
	    </div>
	    `))
	return
    }

    log.Printf("开始提取单词，任务 ID: %s", jobID)

    // 显示加载状态
    c.Data(http.StatusOK, "text/html", []byte(`
	<div class="text-center p-8">
	<span class="spinner"></span>
	<p class="text-gray-600 mt-2">正在提取单词，请稍候...</p>
	</div>
	`))

    // 异步提取单词
    go func() {
	result, err := app.extractor.Extract(c.Request.Context(), job.Result)
	if err != nil {
	    log.Printf("❌ 提取单词失败: %v", err)
	    return
	}

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
	    log.Printf("❌ 保存单词列表失败: %v", err)
	    return
	}

	log.Printf("✓ 成功提取 %d 个单词", len(result.Words))
    }()
}

// handleSyncToMaimemo 同步到墨墨（返回 HTML）
func (app *App) handleSyncToMaimemo(c *gin.Context) {
    jobID := c.Param("job_id")
    token := c.PostForm("token")
    notepadID := c.PostForm("notepad_id")

    if token == "" || notepadID == "" {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="bg-yellow-50 text-yellow-800 p-3 rounded-lg text-sm">
	    ⚠️ 请输入 Token 和云词本 ID
	    </div>
	    `))
	return
    }

    job, err := app.store.Get(jobID)
    if err != nil {
	c.Data(http.StatusNotFound, "text/html", []byte(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 任务不存在
	    </div>
	    `))
	return
    }

    if len(job.Vocabulary) == 0 {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="bg-yellow-50 text-yellow-800 p-3 rounded-lg text-sm">
	    ⚠️ 尚未提取单词，请先提取单词
	    </div>
	    `))
	return
    }

    log.Printf("开始同步到墨墨，任务 ID: %s, 单词数: %d", jobID, len(job.Vocabulary))

    if err := app.maimemoService.AddWordsToNotepad(c.Request.Context(), token, notepadID, job.Vocabulary); err != nil {
	log.Printf("❌ 同步到墨墨失败: %v", err)
	c.Data(http.StatusInternalServerError, "text/html", []byte(fmt.Sprintf(`
	    <div class="bg-red-50 text-red-800 p-3 rounded-lg text-sm">
	    ❌ 同步失败: %v
	    </div>
	    `, err)))
	return
    }

    log.Printf("✓ 成功同步 %d 个单词到墨墨", len(job.Vocabulary))

    c.Data(http.StatusOK, "text/html", []byte(fmt.Sprintf(`
	<div class="bg-green-50 text-green-800 p-3 rounded-lg text-sm">
	✅ 成功同步 %d 个单词到墨墨背单词！
	</div>
	`, len(job.Vocabulary))))
}

// handleListNotepads 查询云词本列表（返回 HTML）
func (app *App) handleListNotepads(c *gin.Context) {
    // 从表单中获取 token（htmx 会自动将 input 值转为 POST 数据）
    token := c.PostForm("token")

    if token == "" {
	c.Data(http.StatusBadRequest, "text/html", []byte(`
	    <div class="p-4 text-center text-yellow-800">
	    ⚠️ 请先输入墨墨 API Token
	    </div>
	    `))
	return
    }

    log.Printf("正在查询云词本列表...")

    notepads, err := app.maimemoService.ListNotepads(c.Request.Context(), token)
    if err != nil {
	log.Printf("❌ 查询云词本列表失败: %v", err)
	c.Data(http.StatusInternalServerError, "text/html", []byte(fmt.Sprintf(`
	    <div class="p-4 text-center text-red-800">
	    ❌ 查询失败: %v
	    </div>
	    `, err)))
	return
    }

    log.Printf("✓ 成功查询到 %d 个云词本", len(notepads))

    // 将 notepads 转换为 map 列表
    notepadMaps := make([]map[string]interface{}, len(notepads))
    for i, notepad := range notepads {
	// 将 notepad 转为 map
	data, _ := json.Marshal(notepad)
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	notepadMaps[i] = m
    }

    // 从 URL 查询参数或表单中获取 jobID（htmx 可以通过 hx-vals 传递）
    jobID := c.Query("job_id")
    if jobID == "" {
	// 尝试从表单中获取
	jobID = c.PostForm("job_id")
    }
    // 如果还是为空，尝试从 Referer 中提取
    if jobID == "" {
	// 假设页面 URL 包含 job_id 信息，或者我们从某个隐藏字段获取
	// 这里我们需要从前端传递，暂时使用一个占位符
	jobID = "unknown"
    }

    html := templates.RenderNotepads(notepadMaps, jobID)
    c.Data(http.StatusOK, "text/html", []byte(html))
}
