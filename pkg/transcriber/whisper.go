package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	whisperAPIURL = "https://api.openai.com/v1/audio/transcriptions"
)

// WhisperClient OpenAI Whisper API 客户端
type WhisperClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewWhisperClient 创建 Whisper 客户端
func NewWhisperClient(apiKey string) *WhisperClient {
	return &WhisperClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // 5 分钟超时
		},
	}
}

// WhisperResponse API 响应
type WhisperResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

// Transcribe 转换音频为文字
// 支持 Context 超时控制（面试亮点）
func (wc *WhisperClient) Transcribe(ctx context.Context, audioPath string, language string) (string, error) {
	// 1. 打开音频文件
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 2. 构造 multipart 表单
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("创建表单失败: %v", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("复制文件失败: %v", err)
	}

	// 添加模型参数
	writer.WriteField("model", "whisper-1")

	// 添加语言参数（可选，不指定则自动检测）
	if language != "" {
		writer.WriteField("language", language)
	}

	// 添加响应格式
	writer.WriteField("response_format", "json")

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭表单失败: %v", err)
	}

	// 3. 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", whisperAPIURL, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+wc.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 4. 发送请求
	resp, err := wc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 5. 检查响应状态
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 返回错误 (状态码 %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// 6. 解析响应
	var whisperResp WhisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	return whisperResp.Text, nil
}

// TranscribeWithRetry 带重试的转换（面试亮点：错误处理）
func (wc *WhisperClient) TranscribeWithRetry(ctx context.Context, audioPath string, language string, maxRetries int) (string, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		text, err := wc.Transcribe(ctx, audioPath, language)
		if err == nil {
			return text, nil
		}

		lastErr = err

		// 检查是否因为 Context 取消
		if ctx.Err() != nil {
			return "", fmt.Errorf("任务被取消: %v", ctx.Err())
		}

		// 指数退避
		if i < maxRetries-1 {
			waitTime := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s, 8s...
			select {
			case <-time.After(waitTime):
				continue
			case <-ctx.Done():
				return "", fmt.Errorf("任务被取消: %v", ctx.Err())
			}
		}
	}

	return "", fmt.Errorf("重试 %d 次后仍然失败: %v", maxRetries, lastErr)
}
