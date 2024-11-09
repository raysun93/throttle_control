package common

import "time"

// Config 系统配置
type Config struct {
	Central     CentralConfig     `json:"central"`
	Application ApplicationConfig `json:"application"`
}

// CentralConfig 中心节点配置
type CentralConfig struct {
	Port             int           `json:"port"`
	MaxTotalQuota    int64         `json:"max_total_quota"`
	MaxQuotaPerNode  int64         `json:"max_quota_per_node"`
	RefreshInterval  time.Duration `json:"refresh_interval"`
	OfflineThreshold time.Duration `json:"offline_threshold"`
	MonitorInterval  time.Duration `json:"monitor_interval"`
}

// ApplicationConfig 应用节点配置
type ApplicationConfig struct {
	Port           int           `json:"port"`
	ReportInterval time.Duration `json:"report_interval"`
	QuotaMargin    float64       `json:"quota_margin"` // 预留配额百分比
	RequestTimeout time.Duration `json:"request_timeout"`
	BatchSize      int           `json:"batch_size"`
	MaxRetries     int           `json:"max_retries"`
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() Config {
	return Config{
		Central: CentralConfig{
			Port:             8080,
			MaxTotalQuota:    1000000,
			MaxQuotaPerNode:  10000,
			RefreshInterval:  5 * time.Second,
			OfflineThreshold: 15 * time.Second,
			MonitorInterval:  5 * time.Second,
		},
		Application: ApplicationConfig{
			Port:           8081,
			ReportInterval: 3 * time.Second,
			QuotaMargin:    0.2, // 20% 预留配额
			RequestTimeout: 2 * time.Second,
			BatchSize:      100,
			MaxRetries:     3,
		},
	}
}
