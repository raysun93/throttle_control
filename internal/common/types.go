package common

import (
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
	Counter     Counter   `json:"counter"`
	LastSeen    time.Time `json:"last_seen"`
	QuotaLeft   int64     `json:"quota_left"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
}

// QuotaRequest 配额请求
type QuotaRequest struct {
	NodeID    string    `json:"node_id"`
	RequestID string    `json:"request_id"`
	Required  int64     `json:"required"`
	Priority  int       `json:"priority"`
	Timestamp time.Time `json:"timestamp"`
}

// QuotaResponse 配额响应
type QuotaResponse struct {
	RequestID  string    `json:"request_id"`
	Granted    int64     `json:"granted"`
	ExpiresAt  time.Time `json:"expires_at"`
	RetryAfter int       `json:"retry_after,omitempty"`
	Message    string    `json:"message,omitempty"`
}
