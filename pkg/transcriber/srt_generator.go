package transcriber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/z-wentao/voiceflow/pkg/models"
)

// SegmentResult 音频片段及其转录结果
type SegmentResult struct {
	Segment  models.Segment
	Response *WhisperResponse
}

// GenerateSRT 生成 SRT 字幕文件
// segments: 音频片段信息（包含时间偏移）
// responses: 对应的 Whisper 响应（包含时间戳）
// outputPath: 输出文件路径
func GenerateSRT(segmentResults []SegmentResult, outputPath string) error {
	// 创建输出目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建 SRT 文件失败: %w", err)
	}
	defer file.Close()

	// 生成 SRT 内容
	var builder strings.Builder
	subtitleIndex := 1

	for _, sr := range segmentResults {
		if sr.Response == nil || len(sr.Response.Segments) == 0 {
			continue
		}

		// 遍历每个 Whisper 片段
		for _, whisperSeg := range sr.Response.Segments {
			// 计算实际时间（加上音频片段的起始偏移）
			actualStart := sr.Segment.Start + whisperSeg.Start
			actualEnd := sr.Segment.Start + whisperSeg.End

			// 格式化 SRT 时间戳
			startTime := formatSRTTime(actualStart)
			endTime := formatSRTTime(actualEnd)

			// 清理文本（去除首尾空格）
			text := strings.TrimSpace(whisperSeg.Text)
			if text == "" {
				continue
			}

			// 写入 SRT 格式
			// 1
			// 00:00:00,000 --> 00:00:05,200
			// 字幕文本
			//
			builder.WriteString(fmt.Sprintf("%d\n", subtitleIndex))
			builder.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))
			builder.WriteString(fmt.Sprintf("%s\n\n", text))

			subtitleIndex++
		}
	}

	// 写入文件
	if _, err := file.WriteString(builder.String()); err != nil {
		return fmt.Errorf("写入 SRT 文件失败: %w", err)
	}

	return nil
}

// formatSRTTime 将秒数格式化为 SRT 时间格式
// 例如: 65.5 -> 00:01:05,500
func formatSRTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}
