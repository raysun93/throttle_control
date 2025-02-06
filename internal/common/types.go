package common

import (
	"context"
	"sync/atomic"
	"time"
)

// NodeState 节点状态
type NodeState int32

const (
	StateUnknown NodeState = iota
	StateOnline
	StateOffline
	StateOverloaded
)

func (s NodeState) String() string {
	switch s {
	case StateOnline:
		return "ONLINE"
	case StateOffline:
		return "OFFLINE"
	case StateOverloaded:
		return "OVERLOADED"
	default:
		return "UNKNOWN"
	}
}

// Counter 请求计数器
type Counter struct {
	Total    atomic.Int64 `json:"total"`
	Accepted atomic.Int64 `json:"accepted"`
	Rejected atomic.Int64 `json:"rejected"`
}

// NodeStatus 节点状态信息
type NodeStatus struct {
	NodeID      string    `json:"node_id"`
	State       NodeState `json:"state"`
	Counter     *Counter  `json:"counter"`
	LastSeen    time.Time `json:"last_seen"`
	QuotaLeft   int64     `json:"quota_left"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
}

// RateControlMethod 速率控制方法
type RateControlMethod int

const (
	RateControlNone RateControlMethod = iota
	RateControlTokenBucket
	RateControlFixedWindow
)

// ProfileQuota 表示单个 profile 的配额请求
type ProfileQuota struct {
	ProfileID int   `json:"profile_id"` // profile 标识
	Required  int64 `json:"required"`   // 请求配额数量
}

// ProfileConfig 定义每个 profile 的配置
type ProfileConfig struct {
	TotalQuota        int64             `json:"total_quota"`         // profile 总配额
	RateLimit         int64             `json:"rate_limit"`          // 每秒最大请求数
	Burst             int64             `json:"burst"`               // 突发请求数
	Description       string            `json:"description"`         // profile 描述
	Window            time.Duration     `json:"window"`              // 速率窗口大小
	RateControlMethod RateControlMethod `json:"rate_control_method"` // 速率控制方法
}

// QuotaRequest 修改后的配额请求
type QuotaRequest struct {
	NodeID    string         `json:"node_id"`
	RequestID string         `json:"request_id"`
	Quotas    []ProfileQuota `json:"quotas"` // 多个 profile 的配额请求
	Timestamp time.Time      `json:"timestamp"`
}

// ProfileQuotaResponse 单个 profile 的配额响应
type ProfileQuotaResponse struct {
	ProfileID   int
	Granted     int64
	Required    int64
	RateLimited bool
}

// QuotaResponse 修改后的配额响应
type QuotaResponse struct {
	RequestID string                 `json:"request_id"`
	Quotas    []ProfileQuotaResponse `json:"quotas"` // 多个 profile 的配额响应
	ExpiresAt time.Time              `json:"expires_at"`
}

// Request represents an incoming request to the node
type Request struct {
	RequestID string
	NodeID    string
	Quotas    map[int]ProfileQuota
}

// Response represents the response from the node
type Response struct {
	RequestID string
	Status    Status
}

// Status represents request processing status
type Status int

const (
	StatusOK Status = iota
	StatusError
	StatusRateLimited
)

// RateLimiter interface defines rate limiting behavior
type RateLimiter interface {
	Allow() bool
}

// Client interface defines communication with central server
type Client interface {
	RequestQuota(ctx context.Context, req QuotaRequest) (QuotaResponse, error)
}

// NodeQuotaStatus represents current node quota status
type NodeQuotaStatus struct {
	NodeID      string
	LastRefresh time.Time
	Quotas      map[int]ProfileStatus
}

// ProfileStatus represents status of a profile's quota
type ProfileStatus struct {
	Allocated int64
	Used      int64
	Available int64
}
