// internal/central/server.go
package central

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"throttle_control/internal/common"
	"time"
)

// Server 中心节点服务器
type Server struct {
	quotaManager *QuotaManager
	config       *ServerConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port            string
	RefreshInterval time.Duration
	ProfileConfigs  map[int]ProfileConfig
}

// NewServer 创建服务器实例
func NewServer(config *ServerConfig) *Server {
	return &Server{
		quotaManager: NewQuotaManager(config.RefreshInterval, config.ProfileConfigs),
		config:       config,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 注册路由
	mux := http.NewServeMux()

	// API路由
	mux.HandleFunc("/api/v1/quota/check", s.handleQuotaCheck)
	mux.HandleFunc("/api/v1/status", s.handleNodeStatus)
	mux.HandleFunc("/health", s.handleHealth)

	// 应用中间件
	handler := s.loggingMiddleware(mux)
	handler = s.recoveryMiddleware(handler)

	server := &http.Server{
		Addr:         s.config.Port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting server on %s", s.config.Port)
	return server.ListenAndServe()
}

// 配额检查处理器
func (s *Server) handleQuotaCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.responseError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req common.QuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.responseError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// 请求验证
	if err := s.validateQuotaRequest(&req); err != nil {
		s.responseError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 处理配额请求
	resp := s.quotaManager.CheckQuota(req)
	s.responseJSON(w, resp)
}

// 节点状态处理器
func (s *Server) handleNodeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.responseError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var status common.NodeStatus
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		s.responseError(w, "Invalid status format", http.StatusBadRequest)
		return
	}

	s.quotaManager.UpdateNodeStatus(status)
	w.WriteHeader(http.StatusOK)
}

// 健康检查处理器
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.responseError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "UP",
		"timestamp": time.Now(),
	}
	s.responseJSON(w, health)
}

// 日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装ResponseWriter以捕获状态码
		wrapper := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapper, r)

		log.Printf(
			"Method: %s | Path: %s | Status: %d | Duration: %v",
			r.Method,
			r.URL.Path,
			wrapper.status,
			time.Since(start),
		)
	})
}

// 恢复中间件
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				s.responseError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// 请求验证
func (s *Server) validateQuotaRequest(req *common.QuotaRequest) error {
	if req.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if len(req.Quotas) == 0 {
		return fmt.Errorf("quotas cannot be empty")
	}
	for _, q := range req.Quotas {
		if q.Required <= 0 {
			return fmt.Errorf("required quota must be positive")
		}
	}
	return nil
}

// JSON响应工具
func (s *Server) responseJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// 错误响应工具
func (s *Server) responseError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ResponseWriter包装器
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
