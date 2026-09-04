package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	SMSVerificationRateLimitMark = "SMSV"
	SMSVerificationMaxRequests   = 3
	SMSVerificationDuration      = 10 * 60
)

func SMSVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		phone := c.Query("phone_number")
		if normalized, err := common.NormalizePhoneNumber(phone); err == nil {
			phone = normalized
		}
		key := fmt.Sprintf("%s:%s:%s", SMSVerificationRateLimitMark, c.ClientIP(), phone)
		if common.RedisEnabled && common.RDB != nil {
			redisKey := "smsVerification:" + key
			count, err := common.RDB.Incr(context.Background(), redisKey).Result()
			if err == nil {
				if count == 1 {
					_ = common.RDB.Expire(context.Background(), redisKey, time.Duration(SMSVerificationDuration)*time.Second).Err()
				}
				if count <= SMSVerificationMaxRequests {
					c.Next()
					return
				}
				ttl, _ := common.RDB.TTL(context.Background(), redisKey).Result()
				waitSeconds := int64(SMSVerificationDuration)
				if ttl > 0 {
					waitSeconds = int64(ttl.Seconds())
				}
				c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", waitSeconds)})
				c.Abort()
				return
			}
		}

		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		if !inMemoryRateLimiter.Request(key, SMSVerificationMaxRequests, SMSVerificationDuration) {
			c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": "发送过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
