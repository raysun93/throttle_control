package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"throttle_control/internal/common"
	"time"
)

// Node represents an application node that manages local quotas
type Node struct {
	nodeID      string
	client      *Client
	mu          sync.RWMutex
	localQuotas map[int]*LocalQuota
	config      NodeConfig
}

// LocalQuota tracks local quota usage and rate limiting
type LocalQuota struct {
	allocated   int64
	used        int64
	lastRefresh time.Time
	rateLimiter RateLimiter
}

// NodeConfig contains node configuration
type NodeConfig struct {
	RefreshInterval time.Duration
	MaxRetries      int
	Timeout         time.Duration
}

// NewNode creates a new application node
func NewNode(nodeID string, client *Client, config NodeConfig) *Node {
	n := &Node{
		nodeID:      nodeID,
		client:      client,
		localQuotas: make(map[int]*LocalQuota),
		config:      config,
	}

	// Start background quota refresh
	go n.startQuotaRefresh()

	return n
}

// HandleRequest processes an incoming request with quota checking
func (n *Node) HandleRequest(req common.Request) (common.Response, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Check local quotas first
	for profileID, quota := range req.Quotas {
		localQuota, exists := n.localQuotas[profileID]
		if !exists {
			return common.Response{}, fmt.Errorf("profile %d not configured", profileID)
		}

		// Check rate limiting
		if !localQuota.rateLimiter.Allow() {
			return common.Response{}, common.ErrRateLimited
		}

		// Check available quota
		if localQuota.allocated-localQuota.used < quota.Required {
			return common.Response{}, common.ErrQuotaExceeded
		}
	}

	// Process request (simulated)
	time.Sleep(100 * time.Millisecond)

	// Update usage
	n.mu.Lock()
	defer n.mu.Unlock()
	for profileID, quota := range req.Quotas {
		n.localQuotas[profileID].used += quota.Required
	}

	return common.Response{
		RequestID: req.RequestID,
		Status:    common.StatusOK,
	}, nil
}

// startQuotaRefresh periodically refreshes quotas from central server
func (n *Node) startQuotaRefresh() {
	ticker := time.NewTicker(n.config.RefreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		n.refreshQuotas()
	}
}

// refreshQuotas fetches and updates local quotas
func (n *Node) refreshQuotas() {
	ctx, cancel := context.WithTimeout(context.Background(), n.config.Timeout)
	defer cancel()

	req := common.QuotaRequest{
		NodeID: n.nodeID,
		Quotas: make(map[int]common.ProfileQuota),
	}

	// Build request with current profiles
	n.mu.RLock()
	for profileID := range n.localQuotas {
		req.Quotas[profileID] = common.ProfileQuota{
			ProfileID: profileID,
			Required:  0, // Just requesting quota refresh
		}
	}
	n.mu.RUnlock()

	// Retry loop
	var resp common.QuotaResponse
	var err error
	for i := 0; i < n.config.MaxRetries; i++ {
		resp, err = n.client.RequestQuota(ctx, req)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		// TODO: Log error and implement fallback strategy
		return
	}

	// Update local quotas
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, profileResp := range resp.Quotas {
		if localQuota, exists := n.localQuotas[profileResp.ProfileID]; exists {
			localQuota.allocated = profileResp.Granted
			localQuota.lastRefresh = time.Now()
		}
	}
}

// GetStatus returns current node status
func (n *Node) GetStatus() common.NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()

	status := common.NodeStatus{
		NodeID:      n.nodeID,
		LastRefresh: time.Now(),
		Quotas:      make(map[int]common.ProfileStatus),
	}

	for profileID, quota := range n.localQuotas {
		status.Quotas[profileID] = common.ProfileStatus{
			Allocated: quota.allocated,
			Used:      quota.used,
			Available: quota.allocated - quota.used,
		}
	}

	return status
}

// HealthCheck performs node health verification
func (n *Node) HealthCheck() error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.localQuotas) == 0 {
		return errors.New("no profiles configured")
	}

	// Check if any quotas haven't been refreshed recently
	for _, quota := range n.localQuotas {
		if time.Since(quota.lastRefresh) > n.config.RefreshInterval*2 {
			return fmt.Errorf("quota refresh stale: last refresh %v", quota.lastRefresh)
		}
	}

	return nil
}
