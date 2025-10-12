package vocabulary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Extractor AI 单词提取器
type Extractor struct {
	client *openai.Client
}

// NewExtractor 创建单词提取器
func NewExtractor(apiKey string) *Extractor {
	return &Extractor{
		client: openai.NewClient(apiKey),
	}
}

// Word 单词信息
type Word struct {
	Word       string `json:"word"`        // 单词
	Definition string `json:"definition"`  // 释义
	Example    string `json:"example"`     // 例句
}

// ExtractResult 提取结果
type ExtractResult struct {
	Words []string `json:"words"` // 单词列表（仅单词，用于墨墨）
	Details []Word `json:"details"` // 详细信息（用于前端展示）
}

// Extract 从文本中提取关键英文单词
func (e *Extractor) Extract(ctx context.Context, text string) (*ExtractResult, error) {
	// 构建 prompt
	prompt := buildPrompt(text)

	// 调用 OpenAI API
	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4oMini, // 使用 GPT-4o-mini，性价比更高
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个专业的英语词汇分析助手。你的任务是从给定的文本中提取重点英文单词，并提供简洁的释义和例句。只返回 JSON 格式的数据，不要有任何其他文字。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.3, // 降低温度，使输出更稳定
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("调用 OpenAI API 失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API 未返回结果")
	}

	// 解析响应
	content := resp.Choices[0].Message.Content
	var result struct {
		Words []Word `json:"words"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w, 原始响应: %s", err, content)
	}

	// 提取单词列表
	words := make([]string, len(result.Words))
	for i, w := range result.Words {
		words[i] = w.Word
	}

	return &ExtractResult{
		Words:   words,
		Details: result.Words,
	}, nil
}

// buildPrompt 构建提示词
func buildPrompt(text string) string {
	// 限制文本长度（避免超出 token 限制）
	const maxLength = 5000
	if len(text) > maxLength {
		text = text[:maxLength] + "..."
	}

	return fmt.Sprintf(`请从以下文本中提取重点英文单词（包括短语）。要求：

1. 提取标准：
   - 选择重要的、值得学习的英文单词和短语
   - 优先选择学术词汇、专业术语、高级词汇
   - 忽略 a, the, is, are 等基础词汇
   - 每个单词只出现一次
   - 最多提取 30 个单词

2. 输出格式（严格遵循 JSON 格式）：
{
  "words": [
    {
      "word": "单词或短语（小写）",
      "definition": "中文释义（简洁，不超过20字）",
      "example": "英文例句（来自原文或自己创建，不超过50字）"
    }
  ]
}

3. 示例：
{
  "words": [
    {
      "word": "artificial intelligence",
      "definition": "人工智能",
      "example": "Artificial intelligence is transforming many industries."
    },
    {
      "word": "sophisticated",
      "definition": "复杂的，精密的",
      "example": "This is a sophisticated algorithm."
    }
  ]
}

文本内容：
%s

请严格按照 JSON 格式输出，不要包含任何其他说明文字。`, text)
}

// FilterDuplicates 去重单词列表
func FilterDuplicates(words []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(words))

	for _, word := range words {
		// 转为小写进行比较
		lower := strings.ToLower(strings.TrimSpace(word))
		if lower == "" {
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			result = append(result, lower)
		}
	}

	return result
}
