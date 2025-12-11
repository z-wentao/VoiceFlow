package transcriber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateVTT 生成 WebVTT 字幕文件（用于 HTML5 video 播放）
func GenerateVTT(segmentResults []SegmentResult, outputPath string) error {
	// 创建输出目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建 VTT 文件失败: %w", err)
	}
	defer file.Close()

	// 生成 VTT 内容
	var builder strings.Builder

	// VTT 文件必须以 "WEBVTT" 开头
	builder.WriteString("WEBVTT\n\n")

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

			// 格式化 VTT 时间戳
			startTime := formatVTTTime(actualStart)
			endTime := formatVTTTime(actualEnd)

			// 清理文本（去除首尾空格）
			text := strings.TrimSpace(whisperSeg.Text)
			if text == "" {
				continue
			}

			// 写入 VTT 格式
			builder.WriteString(fmt.Sprintf("%d\n", subtitleIndex))
			builder.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))
			builder.WriteString(fmt.Sprintf("%s\n\n", text))

			subtitleIndex++
		}
	}

	// 写入文件
	if _, err := file.WriteString(builder.String()); err != nil {
		return fmt.Errorf("写入 VTT 文件失败: %w", err)
	}

	return nil
}

// formatVTTTime 将秒数格式化为 VTT 时间格式
// 例如: 65.5 -> 00:01:05.500
// VTT 使用点号(.)而不是逗号(,)
func formatVTTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
