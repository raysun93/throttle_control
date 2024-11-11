package central

import (
	"fmt"
	"sync"
	"throttle_control/internal/common"
	"time"
)

// ProfileConfig 定义每个 profile 的配置
type ProfileConfig struct {
	TotalQuota  int64  // profile 总配额
	Description string // profile 描述
}

// QuotaManager 支持多 profile 的配额管理器
type QuotaManager struct {
	mu              sync.RWMutex
	profiles        map[int]*ProfileManager // 每个 profile 的管理器
	refreshInterval time.Duration
}

// ProfileManager 单个 profile 的配额管理器
type ProfileManager struct {
	profileID  int
	totalQuota int64
	usedQuota  int64
	nodeQuotas map[string]*NodeQuota
}

// NodeQuota 节点在某个 profile 下的配额信息
type NodeQuota struct {
	allocated int64
	lastCheck time.Time
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(refreshInterval time.Duration, profileConfigs map[int]ProfileConfig) *QuotaManager {
	qm := &QuotaManager{
		profiles:        make(map[int]*ProfileManager),
		refreshInterval: refreshInterval,
	}

	// 初始化每个 profile
	for profileID, config := range profileConfigs {
		qm.profiles[profileID] = &ProfileManager{
			profileID:  profileID,
			totalQuota: config.TotalQuota,
			nodeQuotas: make(map[string]*NodeQuota),
		}
	}

	// 启动周期性更新
	go qm.startPeriodicRefresh()

	return qm
}

// CheckQuota 检查并分配多个 profile 的配额
func (qm *QuotaManager) CheckQuota(req common.QuotaRequest) common.QuotaResponse {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	responses := make([]common.ProfileQuotaResponse, 0, len(req.Quotas))

	// 处理每个 profile 的请求
	for _, profileQuota := range req.Quotas {
		profileMgr, exists := qm.profiles[profileQuota.ProfileID]
		if !exists {
			// 如果 profile 不存在，返回零配额
			responses = append(responses, common.ProfileQuotaResponse{
				ProfileID: profileQuota.ProfileID,
				Granted:   0,
				Required:  profileQuota.Required,
			})
			continue
		}

		// 获取或创建节点配额信息
		nodeQuota, exists := profileMgr.nodeQuotas[req.NodeID]
		if !exists {
			nodeQuota = &NodeQuota{
				lastCheck: time.Now(),
			}
			profileMgr.nodeQuotas[req.NodeID] = nodeQuota
		}

		// 计算可用配额
		remainingQuota := profileMgr.totalQuota - profileMgr.usedQuota
		grantedQuota := profileQuota.Required
		if remainingQuota < profileQuota.Required {
			grantedQuota = remainingQuota
		}

		// 更新配额信息
		if grantedQuota > 0 {
			nodeQuota.allocated += grantedQuota
			profileMgr.usedQuota += grantedQuota
			nodeQuota.lastCheck = time.Now()
		}

		responses = append(responses, common.ProfileQuotaResponse{
			ProfileID: profileQuota.ProfileID,
			Granted:   grantedQuota,
			Required:  profileQuota.Required,
		})
	}

	return common.QuotaResponse{
		RequestID: req.RequestID,
		Quotas:    responses,
		ExpiresAt: time.Now().Add(qm.refreshInterval),
	}
}

// startPeriodicRefresh 开始周期性刷新
func (qm *QuotaManager) startPeriodicRefresh() {
	ticker := time.NewTicker(qm.refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		qm.refresh()
	}
}

// refresh 刷新所有 profile 的配额
func (qm *QuotaManager) refresh() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// 刷新每个 profile 的配额
	for _, profileMgr := range qm.profiles {
		profileMgr.usedQuota = 0
		for _, quota := range profileMgr.nodeQuotas {
			quota.allocated = 0
		}
	}
}

// GetQuotaStatus 获取所有 profile 的配额状态
func (qm *QuotaManager) GetQuotaStatus() map[string]interface{} {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	status := make(map[string]interface{})
	profiles := make(map[string]interface{})

	for profileID, profileMgr := range qm.profiles {
		profileStatus := map[string]interface{}{
			"total_quota": profileMgr.totalQuota,
			"used_quota":  profileMgr.usedQuota,
			"available":   profileMgr.totalQuota - profileMgr.usedQuota,
			"nodes":       make(map[string]interface{}),
		}

		for nodeID, quota := range profileMgr.nodeQuotas {
			profileStatus["nodes"].(map[string]interface{})[nodeID] = map[string]interface{}{
				"allocated": quota.allocated,
				"lastCheck": quota.lastCheck,
			}
		}

		profiles[fmt.Sprintf("profile_%d", profileID)] = profileStatus
	}

	status["profiles"] = profiles
	return status
}
