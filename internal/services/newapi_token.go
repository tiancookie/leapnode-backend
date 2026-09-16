package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UpdateToken 调用 New-API PUT /api/token/ 更新 token。
// New-API 会同步更新 tokens 表 + 失效 Redis 缓存（避免脏缓存）。
//
// 参数 updates 是要更新的字段（支持部分更新），例如：
//   {"status": 2, "name": "new-name"}
//
// 消除技术债：token_service.go 的 UpdateToken 直接写库有缓存一致性风险，
// 改走此方法后 New-API 自己处理缓存失效。
func (c *NewAPIClient) UpdateToken(tokenID int, updates map[string]interface{}) error {
	token, err := c.getAdminToken()
	if err != nil {
		return err
	}

	// New-API PUT /api/token/ 需要完整 token 对象，但我们只传要改的字段
	// 先 GET 拿完整对象，再 merge updates，最后 PUT 回去
	fullToken, err := c.getToken(tokenID)
	if err != nil {
		return err
	}

	// Merge updates
	for k, v := range updates {
		fullToken[k] = v
	}

	body, err := json.Marshal(fullToken)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/token/", c.baseURL)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("New-API update token error: %s", result.Message)
	}
	return nil
}

// getToken 获取 token 完整对象（内部辅助方法，供 UpdateToken 使用）。
func (c *NewAPIClient) getToken(tokenID int) (map[string]interface{}, error) {
	token, err := c.getAdminToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/token/%d", c.baseURL, tokenID)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("New-API get token status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Message string                 `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("New-API get token error: %s", result.Message)
	}
	return result.Data, nil
}

// DeleteToken 调用 New-API DELETE /api/token/:id 删除 token。
// New-API 会同步软删除 tokens 表 + 失效 Redis 缓存（避免已删除的 key 仍能调用 AI）。
//
// 消除技术债：token_service.go 的 DeleteToken 直接写库有缓存一致性风险，
// 改走此方法后 New-API 自己处理缓存失效。
func (c *NewAPIClient) DeleteToken(tokenID int) error {
	token, err := c.getAdminToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/token/%d", c.baseURL, tokenID)
	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("New-API delete token status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("New-API delete token error: %s", result.Message)
	}
	return nil
}
