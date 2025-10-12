package maimemo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// BaseURL 墨墨开放平台 API 基础地址
	BaseURL = "https://open.maimemo.com/open/api/v1"
)

// Client 墨墨背单词 API 客户端
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient 创建墨墨 API 客户端
// token 从墨墨 APP 获取：我的 > 更多设置 > 实验功能 > 开放 API
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Notepad 云词本
type Notepad struct {
	ID          string   `json:"id"`           // 词本ID
	Title       string   `json:"title"`        // 词本标题
	Brief       string   `json:"brief"`        // 简介
	Type        string   `json:"type"`         // 类型
	Status      string   `json:"status"`       // 状态
	Tags        []string `json:"tags"`         // 标签
	Content     string   `json:"content"`      // 词本内容（仅在获取单个词本时返回）
	CreatedTime string   `json:"created_time"` // 创建时间
	UpdatedTime string   `json:"updated_time"` // 更新时间
}

// ListNotepadsResponse 获取云词本列表的响应
type ListNotepadsResponse struct {
	Success bool   `json:"success"`
	Errors  []any  `json:"errors"`
	Data    struct {
		Notepads []Notepad `json:"notepads"`
	} `json:"data"`
}

// GetNotepadResponse 获取单个云词本的响应
type GetNotepadResponse struct {
	Success bool   `json:"success"`
	Errors  []any  `json:"errors"`
	Data    struct {
		Notepad Notepad `json:"notepad"`
	} `json:"data"`
}

// UpdateNotepadRequest 更新云词本的请求
type UpdateNotepadRequest struct {
	Content string `json:"content"` // 词本内容
}

// UpdateNotepadResponse 更新云词本的响应
type UpdateNotepadResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ListNotepads 获取所有云词本
func (c *Client) ListNotepads(ctx context.Context) ([]Notepad, error) {
	url := fmt.Sprintf("%s/notepads", BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result ListNotepadsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 错误: %v", result.Errors)
	}

	return result.Data.Notepads, nil
}

// GetNotepad 获取指定云词本
func (c *Client) GetNotepad(ctx context.Context, notepadID string) (*Notepad, error) {
	url := fmt.Sprintf("%s/notepads/%s", BaseURL, notepadID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	fmt.Printf("GetNotepad 响应: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	var result GetNotepadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 错误: %v", result.Errors)
	}

	return &result.Data.Notepad, nil
}

// UpdateNotepad 更新云词本内容
func (c *Client) UpdateNotepad(ctx context.Context, notepadID string, content string) error {
	url := fmt.Sprintf("%s/notepads/%s", BaseURL, notepadID)

	reqBody := UpdateNotepadRequest{
		Content: content,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	var result UpdateNotepadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("API 错误: %s", result.Message)
	}

	return nil
}

// FormatWordsWithDate 将单词列表格式化为墨墨云词本的格式
// 墨墨要求格式：#20250109\nword1\nword2\n...
func FormatWordsWithDate(words []string, date time.Time) string {
	dateStr := date.Format("20060102")
	content := fmt.Sprintf("#%s\n", dateStr)
	for _, word := range words {
		content += word + "\n"
	}
	return content
}

// AddWordsRequest 添加单词到云词本的请求
type AddWordsRequest struct {
	Words []string `json:"words"` // 单词列表
}

// AddWordsToNotepad 添加单词到云词本（使用 POST 方法，符合官方API规范）
func (c *Client) AddWordsToNotepad(ctx context.Context, notepadID string, words []string) error {
	// 1. 获取现有云词本的完整信息（包括 content）
	fmt.Printf("正在获取云词本详情，ID: %s\n", notepadID)
	targetNotepad, err := c.GetNotepad(ctx, notepadID)
	if err != nil {
		return fmt.Errorf("获取云词本详情失败: %w", err)
	}

	fmt.Printf("当前词本内容长度: %d\n", len(targetNotepad.Content))

	// 2. 格式化新单词
	newContent := FormatWordsWithDate(words, time.Now())

	// 3. 追加到现有内容（如果有的话）
	updatedContent := targetNotepad.Content
	if updatedContent != "" {
		updatedContent += "\n" + newContent
	} else {
		updatedContent = newContent
	}

	fmt.Printf("更新后内容长度: %d\n", len(updatedContent))

	// 4. 构建符合官方API的请求体
	url := fmt.Sprintf("%s/notepads/%s", BaseURL, notepadID)

	reqBody := map[string]interface{}{
		"notepad": map[string]interface{}{
			"status":  targetNotepad.Status,  // 保持原状态
			"content": updatedContent,         // 更新后的内容
			"title":   targetNotepad.Title,   // 保持原标题
			"brief":   targetNotepad.Brief,   // 保持原简介
			"tags":    targetNotepad.Tags,    // 保持原标签
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	fmt.Printf("尝试更新云词本，URL: %s\n", url)
	fmt.Printf("请求体: %s\n", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	fmt.Printf("响应状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应内容: %s\n", string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Success bool  `json:"success"`
		Errors  []any `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("API 错误: %v", result.Errors)
	}

	return nil
}

// AppendWordsToNotepad 将新单词追加到现有云词本
func (c *Client) AppendWordsToNotepad(ctx context.Context, notepadID string, words []string) error {
	// 直接尝试添加单词
	return c.AddWordsToNotepad(ctx, notepadID, words)
}
