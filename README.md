# VoiceFlow - 播客转文字平台

一个基于 Go 语言的高性能语音转文字平台，支持大文件并发处理。

## ✨ 项目亮点（面试加分）

### 核心技术特性

1. **Goroutine Pool 并发控制**
   - 使用 Worker Pool 模式控制并发数量
   - 避免资源耗尽，提升系统稳定性

2. **Channel 通信机制**
   - 任务队列使用 Channel 实现生产者-消费者模式
   - 结果收集通过 Channel 实现并发安全

3. **Context 超时控制**
   - 使用 Context 实现请求超时和优雅取消
   - 支持整个任务链的取消传播

4. **接口抽象设计**
   - 队列接口抽象，方便后续切换到 RabbitMQ
   - 面向接口编程，提升代码可扩展性

5. **音频分片优化**
   - 大文件自动切分（默认 10 分钟/片）
   - 并发转换多个片段，加速处理速度

6. **并发安全存储**
   - 使用 RWMutex 保证存储层并发安全
   - 读写锁分离，提升并发性能

7. **优雅关闭**
   - 信号监听，支持优雅关闭
   - 确保任务处理完成后再退出

8. **AI 集成**（新增）
   - 使用 OpenAI GPT 提取关键英文单词
   - 智能分析文本，生成单词释义和例句

9. **第三方 API 集成**（新增）
   - 集成墨墨背单词开放 API
   - 支持单词自动同步到云词本

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
│   ├── vocabulary/         # 单词提取（新增）
│   │   └── extractor.go    # AI 单词提取器
│   ├── maimemo/            # 墨墨背单词集成（新增）
│   │   └── client.go       # 墨墨 API 客户端
│   ├── worker/             # 任务处理器
│   │   └── worker.go
│   ├── storage/            # 存储层
│   │   └── job_store.go
│   └── config/             # 配置管理
│       └── config.go
├── config/
│   └── config.yaml         # 配置文件
├── web/                    # 前端界面
│   └── index.html
└── uploads/                # 上传文件存储
```

## 🚀 快速开始

### 1. 前置要求

- Go 1.24+
- FFmpeg（用于音频分片）
- OpenAI API Key

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
  worker_count: 3           # 并发 Worker 数量（核心参数）
  segment_duration: 600     # 音频分片时长（秒）
  max_retries: 3            # API 重试次数

# 任务队列配置
queue:
  type: "memory"            # 队列类型: memory 或 rabbitmq
  buffer_size: 100          # 内存队列缓冲区大小

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
加入队列（Channel）
    ↓
Worker 接收任务
    ↓
音频分片（FFmpeg）
    ↓
Goroutine Pool 并发转换
    ↓  (Channel 传递结果)
合并结果
    ↓
保存到存储
    ↓
前端轮询查询结果
```

### 核心组件说明

1. **TranscriptionEngine**（转换引擎）
   - 负责音频分片和并发转换
   - 使用 Goroutine Pool 控制并发数量
   - 通过 Channel 收集转换结果

2. **Worker**（任务处理器）
   - 从队列消费任务
   - 异步处理，不阻塞主线程
   - 支持优雅关闭

3. **Queue**（任务队列）
   - 接口抽象，可切换实现
   - 当前使用内存队列（Channel 实现）
   - 预留 RabbitMQ 接口

4. **JobStore**（存储层）
   - 使用 RWMutex 保证并发安全
   - 内存存储，支持快速查询

## 📈 性能优化

1. **并发处理**：使用 Goroutine Pool 并发转换多个音频片段
2. **音频分片**：大文件切分后并行处理，显著提升速度
3. **读写锁**：存储层使用 RWMutex，提升并发读性能
4. **异步处理**：上传后立即返回，后台异步转换

## 🛠️ 未来扩展

- [x] AI 单词提取功能
- [x] 墨墨背单词集成
- [ ] 接入 RabbitMQ 替换内存队列
- [ ] 添加数据库持久化（PostgreSQL）
- [ ] 支持更多音频格式
- [ ] 添加用户认证系统
- [ ] 实现任务优先级队列
- [ ] 添加 Prometheus 监控指标
- [ ] Docker 容器化部署
- [ ] 支持更多背单词软件（Anki、不背单词等）

## 💡 面试要点

向面试官展示这个项目时，重点说明：

1. **并发设计**
   - "使用 Goroutine Pool 实现并发控制，避免资源耗尽"
   - "通过 Channel 实现生产者-消费者模式，保证并发安全"

2. **扩展性**
   - "使用接口抽象队列，后续可无缝切换到 RabbitMQ"
   - "分层架构设计，各层职责清晰"

3. **性能优化**
   - "音频分片 + 并发转换，提升大文件处理速度"
   - "读写锁优化存储层并发性能"

4. **工程实践**
   - "优雅关闭，确保任务处理完成"
   - "Context 超时控制，避免资源泄漏"
   - "配置文件管理，提升可维护性"

## 📝 技术栈

- **后端**: Go 1.24, Gin Web Framework
- **AI**: OpenAI Whisper API, GPT-4o-mini
- **第三方集成**: 墨墨背单词开放 API
- **音频处理**: FFmpeg
- **前端**: 原生 HTML/CSS/JavaScript
- **并发**: Goroutine, Channel, Context
- **配置**: YAML

## 📄 许可证

MIT License

---

**作者**: [Your Name]
**项目地址**: https://github.com/z-wentao/voiceflow
