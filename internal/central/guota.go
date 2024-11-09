package central

import (
	"sync"
	"throttle_control/internal/common"
	"time"
)

// QuotaManager 配额管理器
type QuotaManager struct {
	mu             sync.RWMutex
	config         common.CentralConfig
	nodeQuotas     map[string]*NodeQuota // 节点配额信息
	totalQuota     int64                 // 系统总配额
	allocatedQuota int64                 // 已分配配额
	lastRebalance  time.Time             // 上次重新平衡时间
}

// NodeQuota 节点配额详细信息
type NodeQuota struct {
	allocated    int64             // 已分配配额
	used         int64             // 已使用配额
	lastUpdate   time.Time         // 最后更新时间
	status       common.NodeStatus // 节点状态
	usageHistory []float64         // 使用率历史记录
}

// NewQuotaManager 创建新的配额管理器
func NewQuotaManager(config common.CentralConfig) *QuotaManager {
	qm := &QuotaManager{
		config:        config,
		nodeQuotas:    make(map[string]*NodeQuota),
		totalQuota:    config.MaxTotalQuota,
		lastRebalance: time.Now(),
	}

	// 启动定期任务
	go qm.startPeriodicTasks()

	return qm
}

// RequestQuota 处理配额请求
func (qm *QuotaManager) RequestQuota(req common.QuotaRequest) common.QuotaResponse {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.nodeQuotas[req.NodeID]
	if !exists {
		quota = &NodeQuota{
			lastUpdate:   time.Now(),
			usageHistory: make([]float64, 0, 10),
			status: common.NodeStatus{
				NodeID: req.NodeID,
				State:  common.StateOnline,
			},
		}
		qm.nodeQuotas[req.NodeID] = quota
	}

	// 检查节点状态
	if quota.status.State != common.StateOnline {
		return common.QuotaResponse{
			RequestID:  req.RequestID,
			Granted:    0,
			RetryAfter: 5,
			Message:    "Node is not online",
		}
	}

	// 计算可分配配额
	availableQuota := qm.calculateAvailableQuota(req.NodeID, quota)
	grantedQuota := min(req.Required, availableQuota)

	if grantedQuota == 0 {
		return common.QuotaResponse{
			RequestID:  req.RequestID,
			Granted:    0,
			RetryAfter: 3,
			Message:    "No quota available",
		}
	}

	// 更新配额信息
	quota.allocated += grantedQuota
	qm.allocatedQuota += grantedQuota
	quota.lastUpdate = time.Now()

	return common.QuotaResponse{
		RequestID: req.RequestID,
		Granted:   grantedQuota,
		ExpiresAt: time.Now().Add(qm.config.RefreshInterval),
	}
}

// calculateAvailableQuota 计算节点可用配额
func (qm *QuotaManager) calculateAvailableQuota(nodeID string, quota *NodeQuota) int64 {
	// 基础配额计算
	baseQuota := qm.config.MaxQuotaPerNode

	// 根据使用率历史调整配额
	if len(quota.usageHistory) > 0 {
		avgUsage := calculateAverage(quota.usageHistory)
		if avgUsage < 0.5 {
			baseQuota = int64(float64(baseQuota) * 0.8) // 降低分配
		} else if avgUsage > 0.8 {
			baseQuota = int64(float64(baseQuota) * 1.2) // 提高分配
		}
	}

	// 考虑系统整体负载
	remainingQuota := qm.totalQuota - qm.allocatedQuota
	if remainingQuota <= 0 {
		return 0
	}

	return min(baseQuota, remainingQuota)
}

// UpdateNodeStatus 更新节点状态
func (qm *QuotaManager) UpdateNodeStatus(status common.NodeStatus) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.nodeQuotas[status.NodeID]
	if !exists {
		quota = &NodeQuota{
			usageHistory: make([]float64, 0, 10),
		}
		qm.nodeQuotas[status.NodeID] = quota
	}

	// 计算使用率
	usageRate := float64(status.Counter.Accepted.Load()) /
		float64(max(status.Counter.Total.Load(), 1))

	// 更新使用率历史
	quota.usageHistory = append(quota.usageHistory, usageRate)
	if len(quota.usageHistory) > 10 {
		quota.usageHistory = quota.usageHistory[1:]
	}

	quota.status = status
	quota.lastUpdate = time.Now()
}

// startPeriodicTasks 启动定期任务
func (qm *QuotaManager) startPeriodicTasks() {
	quotaTimer := time.NewTicker(qm.config.RefreshInterval)
	monitorTimer := time.NewTicker(qm.config.MonitorInterval)

	for {
		select {
		case <-quotaTimer.C:
			qm.rebalanceQuotas()
		case <-monitorTimer.C:
			qm.checkNodeHealth()
		}
	}
}

// rebalanceQuotas 重新平衡配额
func (qm *QuotaManager) rebalanceQuotas() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	activeNodes := 0
	for _, quota := range qm.nodeQuotas {
		if quota.status.State == common.StateOnline {
			activeNodes++
		}
	}

	if activeNodes == 0 {
		return
	}

	baseQuota := qm.totalQuota / int64(activeNodes)
	qm.allocatedQuota = 0

	for _, quota := range qm.nodeQuotas {
		if quota.status.State == common.StateOnline {
			quota.allocated = baseQuota
			qm.allocatedQuota += baseQuota
		} else {
			quota.allocated = 0
		}
	}
}

// checkNodeHealth 检查节点健康状态
func (qm *QuotaManager) checkNodeHealth() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()
	for _, quota := range qm.nodeQuotas {
		if now.Sub(quota.lastUpdate) > qm.config.OfflineThreshold {
			quota.status.State = common.StateOffline
			quota.allocated = 0
		}
	}
}

// 工具函数
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
