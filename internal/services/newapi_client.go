package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// NewAPIClient 封装对 New-API 的 HTTP 调用，避免直接写 users 表。
//
// Token 管理: New-API 的 access_token 15 分钟过期。客户端存 root 账密，
// token 失效时自动重新登录刷新，调用方无需关心 token 生命周期。
type NewAPIClient struct {
	baseURL    string
	httpClient *http.Client

	// root 管理员账密（用于自动登录刷新 token）
	adminUsername string
	adminPassword string

	// token 缓存 + 过期时间 + 并发锁
	mu          sync.Mutex
	adminToken  string
	tokenExpiry time.Time
}

// NewNewAPIClient 创建 New-API 客户端。
func NewNewAPIClient() *NewAPIClient {
	baseURL := os.Getenv("NEW_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://new-api.railway.internal:3000" // Railway 内网默认
	}
	adminUsername := os.Getenv("NEW_API_ADMIN_USERNAME")
	adminPassword := os.Getenv("NEW_API_ADMIN_PASSWORD")
	if adminUsername == "" || adminPassword == "" {
		log.Println("warning: NEW_API_ADMIN_USERNAME/PASSWORD not set, New-API write operations will fail until configured")
	}

	return &NewAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		adminUsername: adminUsername,
		adminPassword: adminPassword,
	}
}

// getAdminToken 返回有效的 admin token，过期或首次调用时自动登录刷新。
// 线程安全。
func (c *NewAPIClient) getAdminToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 提前 60s 刷新，避免边界过期
	if c.adminToken != "" && time.Now().Before(c.tokenExpiry.Add(-60*time.Second)) {
		return c.adminToken, nil
	}

	if c.adminUsername == "" || c.adminPassword == "" {
		return "", fmt.Errorf("New-API admin credentials not configured")
	}

	// 登录刷新
	url := fmt.Sprintf("%s/api/user/login", c.baseURL)
	payload, _ := json.Marshal(map[string]interface{}{
		"username": c.adminUsername,
		"password": c.adminPassword,
	})
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("admin login failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("admin login status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken    string `json:"access_token"`
			AccessExpires  int64  `json:"access_expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if !result.Success || result.Data.AccessToken == "" {
		return "", fmt.Errorf("admin login returned no token")
	}

	c.adminToken = result.Data.AccessToken
	if result.Data.AccessExpires > 0 {
		c.tokenExpiry = time.Unix(result.Data.AccessExpires, 0)
	} else {
		c.tokenExpiry = time.Now().Add(10 * time.Minute) // 保守默认
	}
	log.Printf("New-API admin token refreshed, expires at %s", c.tokenExpiry.Format(time.RFC3339))
	return c.adminToken, nil
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
	// register 是公开接口，无需 token
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
	token, err := c.getAdminToken()
	if err != nil {
		return 0, err
	}
	url := fmt.Sprintf("%s/api/user/search?keyword=%s", c.baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("New-API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// New-API search 返回分页结构 data.items[]，不是直接数组
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				ID       int    `json:"id"`
				Username string `json:"username"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, u := range result.Data.Items {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return 0, fmt.Errorf("user %s not found after creation", username)
}

// manageUserRequest New-API POST /api/user/manage 的请求体。
// 对齐 New-API controller/user.go ManageRequest{Id,Action,Value,Mode}。
type manageUserRequest struct {
	Id     int    `json:"id"`
	Action string `json:"action"` // add_quota / disable / enable / delete / promote / demote
	Value  int    `json:"value"`  // add_quota 时的额度值
	Mode   string `json:"mode"`   // add / subtract (add_quota 时)
}

// IncreaseQuota 增加用户 quota（调用 New-API POST /api/user/manage, action=add_quota mode=add）。
func (c *NewAPIClient) IncreaseQuota(userID int, delta int) error {
	return c.manageUser(manageUserRequest{
		Id:     userID,
		Action: "add_quota",
		Mode:   "add",
		Value:  delta,
	})
}

// DecreaseQuota 扣减用户 quota（action=add_quota mode=subtract）。
func (c *NewAPIClient) DecreaseQuota(userID int, delta int) error {
	return c.manageUser(manageUserRequest{
		Id:     userID,
		Action: "add_quota",
		Mode:   "subtract",
		Value:  delta,
	})
}

// SetUserQuota 设置用户 quota 为指定值（先查当前值算差额，再 add/subtract）。
func (c *NewAPIClient) SetUserQuota(userID int, quota int) error {
	user, err := c.getUser(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	delta := quota - user.Quota
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return c.IncreaseQuota(userID, delta)
	}
	return c.DecreaseQuota(userID, -delta)
}

// manageUser 调用 POST /api/user/manage（需 AdminAuth）。
func (c *NewAPIClient) manageUser(req manageUserRequest) error {
	token, err := c.getAdminToken()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/user/manage", c.baseURL)
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
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
		return fmt.Errorf("New-API manage user error: %s", result.Message)
	}
	return nil
}

// VerifyLogin 通过 New-API 校验用户名密码（调用 POST /api/user/login）。
// 返回用户 ID；密码错误或用户不存在返回 error。
// 用途: LeapNode 密码归 New-API 管理, 登录校验必须委托给 New-API。
func (c *NewAPIClient) VerifyLogin(username, password string) (int, error) {
	url := fmt.Sprintf("%s/api/user/login", c.baseURL)
	payload := map[string]interface{}{
		"username": username,
		"password": password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("login failed (status %d)", resp.StatusCode)
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			User struct {
				ID int `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return 0, fmt.Errorf("invalid credentials: %s", result.Message)
	}
	return result.Data.User.ID, nil
}

// UpdateUserPassword 更新用户密码（调用 New-API PUT /api/user/, body 带 id+password）。
func (c *NewAPIClient) UpdateUserPassword(userID int, newPassword string) error {
	return c.updateUser(userID, map[string]interface{}{
		"id":       userID,
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
	token, err := c.getAdminToken()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/user/%d", c.baseURL, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

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

// updateUser 更新用户信息（调用 PUT /api/user/, body 带 id，需 AdminAuth）。
func (c *NewAPIClient) updateUser(userID int, updates map[string]interface{}) error {
	token, err := c.getAdminToken()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/user/", c.baseURL)
	// 确保 body 里带 id（New-API UpdateUser 从 body 读 id，不是 URL）
	updates["id"] = userID
	body, err := json.Marshal(updates)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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
