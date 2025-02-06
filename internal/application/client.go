// internal/application/client.go
package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"throttle_control/internal/common"
	"time"
)

// CentralClient 中心节点客户端
type CentralClient struct {
	baseURL    string       // 中心节点地址
	httpClient *http.Client // HTTP客户端
	nodeID     string       // 本节点ID
}

// NewCentralClient 创建中心节点客户端
func NewCentralClient(baseURL, nodeID string) *CentralClient {
	return &CentralClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: true,
			},
		},
		nodeID: nodeID,
	}
}

// CheckQuota 请求配额
func (c *CentralClient) CheckQuota(quotas []common.ProfileQuota) (*common.QuotaResponse, error) {
	req := common.QuotaRequest{
		NodeID:    c.nodeID,
		RequestID: fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Quotas:    quotas,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/api/v1/quota/check", c.baseURL),
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("server error: %s", errorResp.Error)
	}

	var quotaResp common.QuotaResponse
	if err := json.NewDecoder(resp.Body).Decode(&quotaResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return &quotaResp, nil
}

// ReportStatus 报告节点状态
func (c *CentralClient) ReportStatus(counter *common.Counter, cpuUsage, memoryUsage float64) error {
	status := common.NodeStatus{
		NodeID:      c.nodeID,
		State:       common.StateOnline,
		Counter:     counter,
		LastSeen:    time.Now(),
		CPUUsage:    cpuUsage,
		MemoryUsage: memoryUsage,
	}

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status failed: %w", err)
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/api/v1/status", c.baseURL),
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("report status failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// GetHealth 检查中心节点健康状态
func (c *CentralClient) GetHealth() error {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/health", c.baseURL))
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy, status code: %d", resp.StatusCode)
	}

	return nil
}

// RetryWithBackoff 重试机制
func (c *CentralClient) RetryWithBackoff(operation func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = operation(); err == nil {
			return nil
		}

		// 指数退避
		backoff := time.Duration(1<<uint(i)) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}

		time.Sleep(backoff)
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}

// Close 关闭客户端
func (c *CentralClient) Close() {
	c.httpClient.CloseIdleConnections()
}

// 使用示例：
func ExampleUsage() {
	client := NewCentralClient("http://localhost:8080", "node1")
	defer client.Close()

	// 检查配额
	quotas := []common.ProfileQuota{
		{ProfileID: 1, Required: 100},
		{ProfileID: 2, Required: 50},
	}

	// 使用重试机制
	err := client.RetryWithBackoff(func() error {
		resp, err := client.CheckQuota(quotas)
		if err != nil {
			return err
		}
		// 处理响应
		fmt.Printf("Quota response: %+v\n", resp)
		return nil
	}, 3)

	if err != nil {
		fmt.Printf("Failed to check quota: %v\n", err)
	}
}
