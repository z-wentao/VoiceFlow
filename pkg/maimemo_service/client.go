package maimemo_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client Maimemo 微服务客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 Maimemo 微服务客户端
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Notepad 云词本
type Notepad struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Brief       string   `json:"brief"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content,omitempty"`
	CreatedTime string   `json:"created_time"`
	UpdatedTime string   `json:"updated_time"`
}

// ListNotepadsResponse 云词本列表响应
type ListNotepadsResponse struct {
	Notepads []Notepad `json:"notepads"`
	Count    int       `json:"count"`
}

// AddWordsRequest 添加单词请求
type AddWordsRequest struct {
	Token     string   `json:"token"`
	NotepadID string   `json:"notepad_id"`
	Words     []string `json:"words"`
}

// AddWordsResponse 添加单词响应
type AddWordsResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// ListNotepads 获取云词本列表
func (c *Client) ListNotepads(ctx context.Context, token string) ([]Notepad, error) {
	url := fmt.Sprintf("%s/api/v1/notepads", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("X-Maimemo-Token", token)
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
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API 错误: %s", errResp.Error)
		}
		return nil, fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	var result ListNotepadsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Notepads, nil
}

// AddWordsToNotepad 添加单词到云词本
func (c *Client) AddWordsToNotepad(ctx context.Context, token, notepadID string, words []string) error {
	url := fmt.Sprintf("%s/api/v1/notepads/%s/words", c.baseURL, notepadID)

	reqBody := AddWordsRequest{
		Token:     token,
		NotepadID: notepadID,
		Words:     words,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

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
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return fmt.Errorf("API 错误: %s", errResp.Error)
		}
		return fmt.Errorf("API 返回错误: %d - %s", resp.StatusCode, string(body))
	}

	var result AddWordsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}
