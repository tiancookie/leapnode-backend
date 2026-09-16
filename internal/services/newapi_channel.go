package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// newAPIChannel 是 New-API channels 表的最小字段集（建 channel 用）。
// New-API 的 model.Channel 有几十个字段，这里只填路由必需的。
type newAPIChannel struct {
	Type    int    `json:"type"`     // 1=OpenAI 兼容
	Name    string `json:"name"`     // 唯一标识（用于反查 id）
	Key     string `json:"key"`      // 上游 API key
	BaseURL string `json:"base_url"` // 上游 base url
	Models  string `json:"models"`   // 逗号分隔的模型列表
	Group   string `json:"group"`    // 分组，默认 "default"
	Status  int    `json:"status"`   // 1=启用
}

type addChannelRequest struct {
	Mode    string        `json:"mode"`
	Channel newAPIChannel `json:"channel"`
}

// CreateChannel 调用 New-API POST /api/channel/ 创建渠道。
// New-API 会在同一事务里建 channels + abilities（AddAbilities），使渠道真正
// 参与 AI 路由。New-API 不返回新 channel 的 id，故创建后按 name 反查返回。
//
// 关键：channels 表由 New-API 独占写，LeapNode 的扩展字段（merchant_id/
// input_price/output_price）由调用方拿到 id 后单独 UPDATE 回写。
func (c *NewAPIClient) CreateChannel(name, key, baseURL, models, group string, channelType int) (int, error) {
	token, err := c.getAdminToken()
	if err != nil {
		return 0, err
	}
	if group == "" {
		group = "default"
	}
	if channelType == 0 {
		channelType = 1 // OpenAI 兼容
	}

	reqBody := addChannelRequest{
		Mode: "",
		Channel: newAPIChannel{
			Type:    channelType,
			Name:    name,
			Key:     key,
			BaseURL: baseURL,
			Models:  models,
			Group:   group,
			Status:  1,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/api/channel/", c.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
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
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return 0, fmt.Errorf("New-API create channel error: %s", result.Message)
	}

	// 反查新建 channel 的 id（按 name）
	return c.getChannelIDByName(name)
}

// getChannelIDByName 按 name 反查 channel id（New-API AddChannel 不返回 id）。
func (c *NewAPIClient) getChannelIDByName(name string) (int, error) {
	token, err := c.getAdminToken()
	if err != nil {
		return 0, err
	}
	url := fmt.Sprintf("%s/api/channel/search?keyword=%s", c.baseURL, name)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("New-API search channel status %d: %s", resp.StatusCode, string(respBody))
	}
	// New-API channel search 返回 {success, data: {items:[...]}} 或 {success, data:[...]}
	var paged struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &paged); err == nil && len(paged.Data.Items) > 0 {
		// 精确匹配 name，取最大 id（最新建的）
		best := 0
		for _, it := range paged.Data.Items {
			if it.Name == name && it.ID > best {
				best = it.ID
			}
		}
		if best > 0 {
			return best, nil
		}
	}
	// 回退：data 为直接数组的老格式
	var flat struct {
		Success bool `json:"success"`
		Data    []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &flat); err == nil {
		best := 0
		for _, it := range flat.Data {
			if it.Name == name && it.ID > best {
				best = it.ID
			}
		}
		if best > 0 {
			return best, nil
		}
	}
	return 0, fmt.Errorf("channel %q not found after create", name)
}

// DeleteChannel 调用 New-API DELETE /api/channel/:id 删除渠道。
// New-API 会同时删 channels + abilities + 刷新缓存。
func (c *NewAPIClient) DeleteChannel(channelID int) error {
	token, err := c.getAdminToken()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/channel/%d", c.baseURL, channelID)
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
		return fmt.Errorf("New-API delete channel status %d: %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("New-API delete channel error: %s", result.Message)
	}
	return nil
}
