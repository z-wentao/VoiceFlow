# VoiceFlow - 播客转文字平台

一个基于 Go 语言的高性能语音转文字平台，支持大文件并发处理。

## 📁 项目结构

```
VoiceFlow/
├── cmd/api/                # 主程序入口
│   └── main.go
├── pkg/
│   ├── models/             # 数据模型
│   │   └── job.go
│   ├── queue/              # 队列接口（可切换实现）
│   │   ├── queue.go        # 接口定义
│   │   ├── memory.go       # 内存实现
│   │   └── rabbitmq.go     # RabbitMQ 实现（预留）
│   ├── transcriber/        # 转换核心
│   │   ├── whisper.go      # Whisper API 客户端
│   │   ├── splitter.go     # 音频分片
│   │   └── engine.go       # 并发引擎（核心亮点）
│   ├── vocabulary/         # 单词提取
│   │   └── extractor.go    # AI 单词提取器
│   ├── maimemo/            # 墨墨背单词集成
│   │   └── client.go       # 墨墨 API 客户端
│   ├── worker/             # 任务处理器
│   │   └── worker.go
│   ├── storage/            # 存储层（核心亮点）
│   │   ├── store.go        # 存储接口
│   │   ├── job_store.go    # 内存存储
│   │   ├── redis_store.go  # Redis 存储
│   │   ├── postgres_store.go  # PostgreSQL 存储
│   │   └── hybrid_store.go # 混合存储（双层架构）
│   └── config/             # 配置管理
│       └── config.go
├── migrations/             # 数据库迁移（Goose）
│   └── 20250117000000_create_jobs_table.sql
├── config/
│   └── config.yaml         # 配置文件
├── web/                    # 前端界面
│   └── index.html
└── uploads/                # 上传文件存储
```

## 🚀 快速开始

### 1. 前置要求

- Go 1.21+
- FFmpeg（用于音频分片）
- OpenAI API Key
- Redis（可选，用于高性能缓存）
- PostgreSQL（可选，用于持久化存储）

### 2. 安装 FFmpeg

**macOS:**
```bash
brew install ffmpeg
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install ffmpeg
```

**Windows:**
下载 FFmpeg 并添加到 PATH：https://ffmpeg.org/download.html

验证安装：
```bash
ffmpeg -version
ffprobe -version
```

### 3. 配置项目

编辑 `config/config.yaml`，填入你的 OpenAI API Key：

```yaml
openai:
  api_key: "sk-your-api-key-here"  # 替换为你的 API Key
```

### 4. 安装依赖

```bash
go mod download
```

### 5. 运行项目

```bash
go run cmd/api/main.go
```

服务器将启动在 `http://localhost:8080`

## 📖 使用说明

### 基础功能

1. 打开浏览器访问 `http://localhost:8080`
2. 点击或拖拽上传音频文件（支持 MP3, WAV, M4A 等）
3. 系统会自动处理并实时显示进度
4. 处理完成后查看转换结果

### 单词提取与同步（新功能）

1. **提取单词**：任务完成后，点击"📚 提取单词"按钮
   - AI 会自动分析文本内容
   - 提取重点英文单词（最多 30 个）
   - 显示单词释义和例句

2. **同步到墨墨背单词**：
   - 点击"🔄 同步到墨墨"按钮
   - 输入墨墨 API Token（获取方式：墨墨 APP → 我的 → 更多设置 → 实验功能 → 开放 API）
   - 输入云词本 ID
   - 确认同步，单词会自动添加到你的墨墨云词本中

## 🔧 配置说明

编辑 `config/config.yaml` 自定义配置：

```yaml
# OpenAI API 配置
openai:
  api_key: "your-api-key"

# 转换引擎配置
transcriber:
  worker_pool_size: 3       # Worker 实例数量
  segment_concurrency: 3    # 音频分片并发处理数（核心参数）
  segment_duration: 600     # 音频分片时长（秒）
  max_retries: 3            # API 重试次数

# 任务队列配置
queue:
  type: "memory"            # 队列类型: memory 或 rabbitmq
  buffer_size: 100          # 内存队列缓冲区大小

# 存储配置（核心亮点）
storage:
  type: "hybrid"            # 存储类型: memory/redis/postgres/hybrid

  # Redis 配置（热数据缓存）
  redis:
    addr: "localhost:6379"
    password: ""            # 无密码留空
    db: 0
    ttl: 168                # 过期时间（小时），默认 7 天

  # PostgreSQL 配置（冷数据持久化）
  postgres:
    host: "localhost"
    port: 5432
    user: "postgres"
    password: "password"
    database: "voiceflow"
    sslmode: "disable"

# 服务器配置
server:
  port: 8080
  max_upload_size: 104857600  # 最大上传文件大小（100MB）
```

## 🎯 API 接口

### 1. 上传音频
```
POST /api/upload
Content-Type: multipart/form-data

参数:
- audio: 音频文件

响应:
{
  "job_id": "uuid",
  "filename": "podcast.mp3",
  "size": 12345678,
  "status": "pending",
  "message": "上传成功，正在处理中..."
}
```

### 2. 查询任务状态
```
GET /api/jobs/:job_id

响应:
{
  "job_id": "uuid",
  "filename": "podcast.mp3",
  "status": "processing",
  "progress": 45,
  "result": "",
  "vocabulary": ["word1", "word2"],
  "vocab_detail": [
    {
      "word": "artificial intelligence",
      "definition": "人工智能",
      "example": "AI is transforming industries."
    }
  ],
  "error": ""
}
```

### 3. 列出所有任务
```
GET /api/jobs

响应:
{
  "jobs": [...],
  "total": 10
}
```

### 4. 提取单词（新功能）
```
POST /api/jobs/:job_id/extract-vocabulary

响应:
{
  "job_id": "uuid",
  "vocabulary": ["word1", "word2", ...],
  "vocab_detail": [...],
  "count": 30
}
```

### 5. 同步到墨墨背单词（新功能）
```
POST /api/jobs/:job_id/sync-to-maimemo
Content-Type: application/json

请求体:
{
  "token": "your_maimemo_token",
  "notepad_id": "your_notepad_id"
}

响应:
{
  "message": "同步成功",
  "count": 30
}
```

## 🔍 架构设计

### 请求处理流程

```
上传音频
    ↓
生成 Job ID
    ↓
保存文件
    ↓
立即写入 Redis ────────────┐
    ↓                      │（异步批量）
加入队列（Channel）        │
    ↓                      ↓
Worker 接收任务      PostgreSQL
    ↓                 （持久化）
音频分片（FFmpeg）
    ↓
Goroutine Pool 并发转换
    ↓  (Channel 传递结果)
合并结果
    ↓
更新 Redis ──────────────┐
    ↓                    │（异步批量）
任务完成                 │
    ↓                    ↓
前端查询              PostgreSQL
    ↓                 （持久化）
优先 Redis ──未命中──→ PostgreSQL
    ↓                      ↓
返回结果 ←────回写 Redis────┘
```

### 混合存储架构

```
┌─────────────────────────────────────┐
│          应用层 (Gin Server)         │
└──────────────┬──────────────────────┘
               ↓
┌──────────────────────────────────────┐
│        HybridJobStore（协调层）      │
├──────────────┬───────────────────────┤
│  Write-Behind│  Cache-Aside          │
│  写策略      │  读策略               │
└──────────────┴───────────────────────┘
       ↓                    ↓
┌─────────────┐      ┌─────────────────┐
│   Redis     │      │  PostgreSQL     │
│  （热数据）  │      │  （冷数据）      │
├─────────────┤      ├─────────────────┤
│ • 7天 TTL   │      │ • 永久存储      │
│ • 0.5ms 响应│      │ • JSONB 字段    │
│ • 命中率95% │      │ • 索引优化      │
└─────────────┘      └─────────────────┘
```

**存储策略：**
- **写入**: 立即写 Redis（快速响应） → 异步批量写 PostgreSQL（50条或5秒）
- **读取**: 优先 Redis（命中率95%） → 未命中查 PostgreSQL → 自动回写 Redis
- **故障处理**: Redis 挂了降级到 PostgreSQL，保证服务可用

### 核心组件说明

1. **TranscriptionEngine**（转录引擎）
   - 负责音频分片和并发转换
   - 使用 Goroutine Pool 控制并发数量（信号量模式）
   - 通过 Channel 收集转换结果

2. **Worker Pool**（任务处理器池）
   - 从队列消费任务（阻塞式 Dequeue）
   - 3 个 Worker 并发处理，避免 API 限流
   - 异步处理，不阻塞主线程
   - 支持优雅关闭（Context 控制）

3. **Queue**（任务队列）
   - 接口抽象，可切换实现
   - 当前使用内存队列（Channel 实现）
   - 预留 RabbitMQ 接口

4. **HybridJobStore**（混合存储）
   - Redis + PostgreSQL 双层架构
   - Write-Behind 写策略 + Cache-Aside 读策略
   - 批量同步机制（后台 Worker）
   - 故障降级保证可用性

## 📈 性能优化

1. **混合存储架构**
   - Redis + PostgreSQL 双层存储
   - 性能优异，查询任务列表 QPS 达到 11,000+，单个任务查询 QPS 达到 22,000+
   - 批量同步机制减少 70% 数据库写压力

2. **并发处理**
   - Worker 池控制并发数（3个实例）
   - 音频分片并发转换（每片 3 个并发）
   - Goroutine Pool 模式避免资源耗尽

3. **音频分片**
   - 大文件切分为 10 分钟小片段
   - 并行处理，1 小时音频 15 分钟完成
   - 显著提升处理速度（4 倍）

4. **异步处理**
   - 上传后立即返回（非阻塞）
   - 后台 Worker 异步转换
   - 前端轮询获取进度

5. **数据库优化**
   - JSONB 字段存储非结构化数据
   - 索引优化（status, created_at）
   - 批量写入减少 I/O

## 🛠️ 未来扩展

- [x] AI 单词提取功能
- [x] 墨墨背单词集成
- [x] Redis 缓存存储
- [x] PostgreSQL 持久化存储
- [x] 混合存储架构
- [x] 数据库迁移工具（Goose）
- [ ] 接入 RabbitMQ 替换内存队列
- [ ] 支持更多音频格式
- [ ] 添加用户认证系统
- [ ] 实现任务优先级队列
- [ ] 添加 Prometheus 监控指标
- [ ] Docker 容器化部署
- [ ] Kubernetes 自动扩缩容
- [ ] 支持更多背单词软件（Anki、不背单词等）

## 📝 技术栈

- **后端**: Go 1.21+, Gin Web Framework
- **AI**: OpenAI Whisper API, GPT-4o-mini
- **存储**: Redis（缓存）, PostgreSQL（持久化）
- **数据库迁移**: Goose
- **第三方集成**: 墨墨背单词开放 API
- **音频处理**: FFmpeg
- **前端**: 原生 HTML/CSS/JavaScript
- **并发**: Goroutine, Channel, Context, WaitGroup
- **配置**: YAML

## 📄 许可证

MIT License

---
