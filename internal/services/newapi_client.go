package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// NewAPIClient 封装对 New-API 的 HTTP 调用，避免直接写 users 表。
type NewAPIClient struct {
	baseURL    string
	httpClient *http.Client
	// 管理员 token（从环境变量读取，用于调用 New-API 管理接口）
	adminToken string
}

// NewNewAPIClient 创建 New-API 客户端。
func NewNewAPIClient() *NewAPIClient {
	baseURL := os.Getenv("NEW_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://new-api.railway.internal:3000" // Railway 内网默认
	}
	adminToken := os.Getenv("NEW_API_ADMIN_TOKEN")
	if adminToken == "" {
		panic("NEW_API_ADMIN_TOKEN environment variable is required")
	}

	return &NewAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		adminToken: adminToken,
	}
}

// CreateUser 通过 New-API 创建用户（POST /api/user/register 或管理接口）。
// 返回创建后的用户 ID。
func (c *NewAPIClient) CreateUser(username, password, email string) (int, error) {
	url := fmt.Sprintf("%s/api/user/register", c.baseURL)
	payload := map[string]interface{}{
		"username": username,
		"password": password,
		"email":    email,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("New-API error: %s", result.Message)
	}

	// New-API register 接口可能不返回 id，需要按 username 反查
	if result.Data.ID == 0 {
		return c.getUserIDByUsername(username)
	}

	return result.Data.ID, nil
}

// getUserIDByUsername 按用户名反查用户 ID。
func (c *NewAPIClient) getUserIDByUsername(username string) (int, error) {
	url := fmt.Sprintf("%s/api/user/search?keyword=%s", c.baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, u := range result.Data {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return 0, fmt.Errorf("user %s not found after creation", username)
}

// IncreaseQuota 增加用户 quota（调用 New-API 的 PUT /api/user/:id 接口）。
func (c *NewAPIClient) IncreaseQuota(userID int, delta int) error {
	// 1. 先获取当前用户信息
	user, err := c.getUser(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// 2. 计算新 quota
	newQuota := user.Quota + delta

	// 3. 调用更新接口
	return c.updateUser(userID, map[string]interface{}{
		"quota": newQuota,
	})
}

// SetUserQuota 设置用户 quota 为指定值。
func (c *NewAPIClient) SetUserQuota(userID int, quota int) error {
	return c.updateUser(userID, map[string]interface{}{
		"quota": quota,
	})
}

// UpdateUserPassword 更新用户密码（调用 New-API 的 PUT /api/user/:id 接口）。
func (c *NewAPIClient) UpdateUserPassword(userID int, newPassword string) error {
	return c.updateUser(userID, map[string]interface{}{
		"password": newPassword,
	})
}

// userInfo 从 New-API 返回的用户信息。
type userInfo struct {
	ID    int `json:"id"`
	Quota int `json:"quota"`
}

// getUser 获取用户信息（调用 GET /api/user/:id）。
func (c *NewAPIClient) getUser(userID int) (*userInfo, error) {
	url := fmt.Sprintf("%s/api/user/%d", c.baseURL, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool      `json:"success"`
		Data    *userInfo `json:"data"`
		Message string    `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("New-API error: %s", result.Message)
	}

	return result.Data, nil
}

// updateUser 更新用户信息（调用 PUT /api/user/:id）。
func (c *NewAPIClient) updateUser(userID int, updates map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/user/%d", c.baseURL, userID)
	body, err := json.Marshal(updates)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("New-API error: %s", result.Message)
	}

	return nil
}
