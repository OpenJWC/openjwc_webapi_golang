package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// loginBucket 保存单个来源地址的登录限流状态。
type loginBucket struct {
	tokens  float64
	updated time.Time
}

// loginLimiter 用有界地址表保护昂贵密码计算，不信任转发请求头。
type loginLimiter struct {
	mutex   sync.Mutex
	buckets map[string]loginBucket
}

// allow 为当前地址补充令牌并限制地址表增长。
func (limiter *loginLimiter) allow(address string, current time.Time) bool {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	bucket, found := limiter.buckets[address]
	if !found {
		if len(limiter.buckets) >= 4096 {
			for key, value := range limiter.buckets {
				if current.Sub(value.updated) > time.Minute {
					delete(limiter.buckets, key)
				}
			}
			if len(limiter.buckets) >= 4096 {
				return false
			}
		}
		bucket = loginBucket{tokens: 20, updated: current}
	}
	bucket.tokens = min(20, bucket.tokens+current.Sub(bucket.updated).Seconds()*2)
	bucket.updated = current
	allowed := bucket.tokens >= 1
	if allowed {
		bucket.tokens--
	}
	limiter.buckets[address] = bucket
	return allowed
}

// limitLogins 为注册和登录增加每来源地址限流，其他接口不受此限制。
func limitLogins(next http.Handler) http.Handler {
	limiter := &loginLimiter{buckets: make(map[string]loginBucket)}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/v2/client/auth/") {
			address, _, err := net.SplitHostPort(request.RemoteAddr)
			if err != nil {
				address = request.RemoteAddr
			}
			if !limiter.allow(address, time.Now()) {
				writer.Header().Set("Retry-After", "1")
				writeJSON(writer, 429, map[string]string{"detail": "登录或注册请求过于频繁"})
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
