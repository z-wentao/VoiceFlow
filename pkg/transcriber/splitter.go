package transcriber

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/z-wentao/voiceflow/pkg/models"
)

// AudioSplitter 音频分片器
type AudioSplitter struct {
	segmentDuration int // 每个片段的时长（秒），默认 600 秒（10 分钟）
}

// NewAudioSplitter 创建分片器
func NewAudioSplitter(segmentDuration int) *AudioSplitter {
	if segmentDuration <= 0 {
		segmentDuration = 600 // 默认 10 分钟
	}
	return &AudioSplitter{
		segmentDuration: segmentDuration,
	}
}

// Split 将音频文件切分成多个片段
// 面试亮点：处理大文件，优化并发转换
func (as *AudioSplitter) Split(audioPath string) ([]models.Segment, error) {
	// 1. 获取音频时长
	duration, err := as.getAudioDuration(audioPath)
	if err != nil {
		return nil, fmt.Errorf("获取音频时长失败: %v", err)
	}

	// 2. 计算需要切分的片段数
	segmentCount := int(duration)/as.segmentDuration + 1
	log.Printf("📊 音频时长: %.2f 秒 (%.2f 分钟)", duration, duration/60)

	if duration <= float64(as.segmentDuration) {
		// 不需要切分，直接返回原文件
		log.Printf("✓ 音频较短，无需切分，直接处理")
		return []models.Segment{
			{
				Index:    0,
				FilePath: audioPath,
				Start:    0,
				End:      duration,
			},
		}, nil
	}

	log.Printf("✂️  音频将被切分为 %d 个片段 (每片 %d 秒)", segmentCount, as.segmentDuration)

	// 3. 创建临时目录存放片段
	segmentsDir := filepath.Join(filepath.Dir(audioPath), "segments")
	if err := os.MkdirAll(segmentsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建片段目录失败: %v", err)
	}

	// 4. 切分音频
	segments := make([]models.Segment, 0, segmentCount)
	for i := 0; i < segmentCount; i++ {
		start := float64(i * as.segmentDuration)
		end := start + float64(as.segmentDuration)
		if end > duration {
			end = duration
		}

		// 片段文件名
		segmentPath := filepath.Join(segmentsDir, fmt.Sprintf("segment_%03d.mp3", i))

		// 使用 FFmpeg 切分
		log.Printf("  ✂️  正在切分片段 %d/%d: %.2f秒 -> %.2f秒 (时长: %.2f秒)",
			i+1, segmentCount, start, end, end-start)
		if err := as.extractSegment(audioPath, segmentPath, start, float64(as.segmentDuration)); err != nil {
			return nil, fmt.Errorf("切分片段 %d 失败: %v", i, err)
		}

		segments = append(segments, models.Segment{
			Index:    i,
			FilePath: segmentPath,
			Start:    start,
			End:      end,
		})
	}

	return segments, nil
}

// getAudioDuration 获取音频/视频文件时长（秒）
func (as *AudioSplitter) getAudioDuration(audioPath string) (float64, error) {
	// 使用 FFprobe 获取时长
	// ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 input.mp3
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)

	// 捕获 stdout 和 stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe 执行失败: %v (stderr: %s)", err, stderr.String())
	}

	durationStr := strings.TrimSpace(stdout.String())
	if durationStr == "" {
		return 0, fmt.Errorf("ffprobe 未返回时长信息 (stderr: %s)", stderr.String())
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析时长失败: %v (output: %s)", err, durationStr)
	}

	return duration, nil
}

// extractSegment 从音频/视频中提取片段
func (as *AudioSplitter) extractSegment(inputPath, outputPath string, startTime, duration float64) error {
	// 判断输入文件类型
	ext := strings.ToLower(filepath.Ext(inputPath))
	isVideo := (ext == ".mp4" || ext == ".webm" || ext == ".avi" || ext == ".mov")

	var cmd *exec.Cmd

	if isVideo {
		// 视频文件：提取音频并转码为 MP3
		// ffmpeg -i video.mp4 -ss 0 -t 300 -vn -acodec libmp3lame -ab 128k -y output.mp3
		cmd = exec.Command("ffmpeg",
			"-i", inputPath,
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%.2f", duration),
			"-vn",              // 禁用视频流
			"-acodec", "libmp3lame", // 转码为 MP3
			"-ab", "128k",      // 音频比特率 128kbps
			"-y",
			outputPath,
		)
	} else {
		// 纯音频文件：直接复制（快速，不重新编码）
		// ffmpeg -i input.mp3 -ss 0 -t 300 -acodec copy -y output.mp3
		cmd = exec.Command("ffmpeg",
			"-i", inputPath,
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%.2f", duration),
			"-acodec", "copy",
			"-y",
			outputPath,
		)
	}

	// 捕获 stderr 以便调试
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg 执行失败: %v (stderr: %s)", err, stderr.String())
	}

	return nil
}

// Cleanup 清理临时片段文件
func (as *AudioSplitter) Cleanup(segments []models.Segment) error {
	if len(segments) > 0 {
		segmentsDir := filepath.Dir(segments[0].FilePath)
		// 只删除临时创建的 segments 目录，不删除 uploads 等原始目录
		// 通过检查目录名是否为 "segments" 来判断
		if filepath.Base(segmentsDir) == "segments" {
			log.Printf("🧹 清理临时片段目录: %s", segmentsDir)
			return os.RemoveAll(segmentsDir)
		}
		log.Printf("✓ 跳过清理原始文件目录: %s", segmentsDir)
	}
	return nil
}
