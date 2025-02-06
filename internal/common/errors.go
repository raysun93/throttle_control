package common

import "errors"

var (
	ErrNoQuota        = errors.New("no quota available")
	ErrNodeOffline    = errors.New("node is offline")
	ErrRequestTimeout = errors.New("request timeout")
	ErrOverloaded     = errors.New("system overloaded")
	ErrInvalidRequest = errors.New("invalid request")
	ErrQuotaExceeded  = errors.New("quota exceeded")
	ErrNodeNotFound   = errors.New("node not found")
	ErrRateLimited    = errors.New("rate limited")
)
