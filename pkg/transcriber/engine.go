package transcriber

import (
    "context"
    "fmt"
    "log"
    "path/filepath"
    "sort"
    "strings"
    "sync"

    "github.com/z-wentao/voiceflow/pkg/models"
)

// TranscriptionEngine 转换引擎
// 面试亮点：Goroutine Pool + Channel 并发处理
type TranscriptionEngine struct {
    whisperClient       *WhisperClient
    splitter            *AudioSplitter
    segmentConcurrency  int // 音频分片并发处理数
}

func NewTranscriptionEngine(apiKey string, segmentConcurrency int, segmentDuration int) *TranscriptionEngine {
    if segmentConcurrency <= 0 {
	segmentConcurrency = 3 // 默认 3 个并发分片处理
    }

    return &TranscriptionEngine{
	whisperClient:      NewWhisperClient(apiKey),
	splitter:           NewAudioSplitter(segmentDuration),
	segmentConcurrency: segmentConcurrency,
    }
}

// ProcessResult 处理结果（内部用于 Channel 传递）
type ProcessResult struct {
    SegmentIndex int
    Response     *WhisperResponse // 完整的 Whisper 响应（包含时间戳）
    Error        error
}

// TranscriptionResult 转录结果
type TranscriptionResult struct {
    Text         string // 纯文本结果
    SubtitlePath string // SRT 字幕文件路径
    VTTPath      string // WebVTT 字幕文件路径（用于网页播放）
}

// Transcribe 转换整个音频文件（返回文本和字幕）
// 1. 使用 Context 控制超时和取消
// 2. Goroutine Pool 控制并发数
// 3. Channel 收集结果
// 4. WaitGroup 等待所有 Goroutine 完成
// 5. 错误处理和进度回调
func (te *TranscriptionEngine) Transcribe(
    ctx context.Context,
    audioPath string,
    language string,
    progressCallback func(progress int),
) (*TranscriptionResult, error) {
    // split the video or audio
    log.Printf("开始分片音频: %s", audioPath)
    segments, err := te.splitter.Split(audioPath)
    if err != nil {
	return nil, fmt.Errorf("分片失败: %v", err)
    }
    defer te.splitter.Cleanup(segments)

    totalSegments := len(segments)
    log.Printf("✓ 音频已分片，共 %d 个片段", totalSegments)

    // 2. 创建任务队列和结果收集 Channel
    taskChan := make(chan models.Segment, totalSegments)
    resultChan := make(chan ProcessResult, totalSegments)

    // 3. 启动 Goroutine Pool（面试亮点：并发控制）
    log.Printf("🚀 启动 %d 个并发分片处理器进行处理...", te.segmentConcurrency)
    var wg sync.WaitGroup
    for i := 0; i < te.segmentConcurrency; i++ {
	wg.Add(1)
	go te.segmentProcessor(ctx, i, taskChan, resultChan, language, &wg)
    }

    // 4. 发送任务到队列
    for _, segment := range segments {
	taskChan <- segment
    }
    close(taskChan) // 关闭任务 Channel，告诉 worker 没有更多任务了

    // 5. 启动结果收集 Goroutine
    go func() {
	wg.Wait()           // 等待所有 worker 完成
	close(resultChan)   // 关闭结果 Channel
    }()

    // 6. 收集结果
    results := make(map[int]*WhisperResponse)
    var errors []error
    completedCount := 0

    for result := range resultChan {
	completedCount++

	if result.Error != nil {
	    errors = append(errors, fmt.Errorf("片段 %d 失败: %v", result.SegmentIndex, result.Error))
	    log.Printf("❌ 片段 #%d 转换失败: %v", result.SegmentIndex, result.Error)
	} else {
	    results[result.SegmentIndex] = result.Response
	    log.Printf("✅ 片段 #%d 转换完成 | 进度: %d/%d (%.1f%%) | 文本长度: %d 字符",
		result.SegmentIndex, completedCount, totalSegments,
		float64(completedCount*100)/float64(totalSegments), len(result.Response.Text))
	}

	// 进度回调
	if progressCallback != nil {
	    progress := (completedCount * 100) / totalSegments
	    progressCallback(progress)
	}
    }

    // 7. 检查是否有错误
    if len(errors) > 0 {
	return nil, fmt.Errorf("转换过程中出现 %d 个错误: %v", len(errors), errors[0])
    }

    // 8. 按顺序合并文本结果
    finalText := te.mergeTextResults(results, totalSegments)
    log.Printf("✓ 所有片段转换完成，总长度: %d 字符", len(finalText))

    // 9. 生成字幕文件（SRT 和 VTT）
    srtPath, vttPath, err := te.generateSubtitleFiles(segments, results, audioPath)
    if err != nil {
	log.Printf("⚠️ 生成字幕文件失败: %v", err)
	// 不影响主流程，继续返回文本结果
	return &TranscriptionResult{
	    Text:         finalText,
	    SubtitlePath: "",
	    VTTPath:      "",
	}, nil
    }

    log.Printf("✓ 字幕文件已生成:")
    log.Printf("  - SRT: %s", srtPath)
    log.Printf("  - VTT: %s", vttPath)
    return &TranscriptionResult{
	Text:         finalText,
	SubtitlePath: srtPath,
	VTTPath:      vttPath,
    }, nil
}

// segmentProcessor 分片处理器 - Goroutine Pool 中的工作单元
// 面试亮点：展示 Goroutine、Channel 和 Context 的配合使用
func (te *TranscriptionEngine) segmentProcessor(
    ctx context.Context,
    processorID int,
    taskChan <-chan models.Segment,
    resultChan chan<- ProcessResult,
    language string,
    wg *sync.WaitGroup,
) {
    defer wg.Done()

    log.Printf("分片处理器 #%d 启动", processorID)

    for segment := range taskChan {
	// 检查 Context 是否已取消
	select {
	case <-ctx.Done():
	    resultChan <- ProcessResult{
		SegmentIndex: segment.Index,
		Error:        fmt.Errorf("任务被取消"),
	    }
	    return
	default:
	}

	// 转换音频片段（带重试）
	log.Printf("🔄 [分片处理器-%d] 正在处理片段 #%d (%.1fs - %.1fs)",
	    processorID, segment.Index, segment.Start, segment.End)
	response, err := te.whisperClient.TranscribeWithRetry(ctx, segment.FilePath, language, 3)

	// 发送结果
	resultChan <- ProcessResult{
	    SegmentIndex: segment.Index,
	    Response:     response,
	    Error:        err,
	}
    }

    log.Printf("分片处理器 #%d 结束", processorID)
}

// mergeTextResults 按顺序合并所有片段的文本结果
func (te *TranscriptionEngine) mergeTextResults(results map[int]*WhisperResponse, totalSegments int) string {
    // 按索引排序
    indices := make([]int, 0, len(results))
    for idx := range results {
	indices = append(indices, idx)
    }
    sort.Ints(indices)

    // 合并文本
    var builder strings.Builder
    for _, idx := range indices {
	if idx > 0 {
	    builder.WriteString(" ") // 片段之间添加空格
	}
	if resp := results[idx]; resp != nil {
	    builder.WriteString(resp.Text)
	}
    }

    return builder.String()
}

// generateSubtitleFiles 生成字幕文件（SRT 和 VTT）
func (te *TranscriptionEngine) generateSubtitleFiles(
    segments []models.Segment,
    results map[int]*WhisperResponse,
    audioPath string,
) (string, string, error) {
    // 准备 SegmentResult 数据
    segmentResults := make([]SegmentResult, 0, len(segments))
    for _, seg := range segments {
	if resp, ok := results[seg.Index]; ok {
	    segmentResults = append(segmentResults, SegmentResult{
		Segment:  seg,
		Response: resp,
	    })
	}
    }

    // 确定输出路径（与音频文件同目录）
    basePath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))
    srtPath := basePath + ".srt"
    vttPath := basePath + ".vtt"

    // 生成 SRT 文件
    if err := GenerateSRT(segmentResults, srtPath); err != nil {
	return "", "", fmt.Errorf("生成 SRT 失败: %w", err)
    }

    // 生成 VTT 文件
    if err := GenerateVTT(segmentResults, vttPath); err != nil {
	return "", "", fmt.Errorf("生成 VTT 失败: %w", err)
    }

    return srtPath, vttPath, nil
}
