package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
)

const (
	SMSVerificationRateLimitMark = "SMSV"
	SMSVerificationMaxRequests   = 3
	SMSVerificationDuration      = 10 * 60
	SMSVerificationIPMaxRequests = 20
)

func abortSMSVerificationUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, i18n.MsgUserSMSVerificationUnavailable),
	})
	c.Abort()
}

func SMSVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			PhoneNumber string `json:"phone_number"`
		}
		_ = common.UnmarshalBodyReusable(c, &request)
		phone := request.PhoneNumber
		if normalized, err := common.NormalizePhoneNumber(phone); err == nil {
			phone = normalized
		}
		ip := c.ClientIP()
		keys := []struct {
			name string
			max  int64
		}{
			{name: fmt.Sprintf("%s:ip:%s", SMSVerificationRateLimitMark, ip), max: SMSVerificationIPMaxRequests},
			{name: fmt.Sprintf("%s:phone:%s", SMSVerificationRateLimitMark, phone), max: SMSVerificationMaxRequests},
			{name: fmt.Sprintf("%s:pair:%s:%s", SMSVerificationRateLimitMark, ip, phone), max: SMSVerificationMaxRequests},
		}
		if common.RedisEnabled {
			if common.RDB == nil {
				abortSMSVerificationUnavailable(c)
				return
			}
			for _, item := range keys {
				redisKey := "smsVerification:" + item.name
				count, err := common.RDB.Incr(context.Background(), redisKey).Result()
				if err != nil {
					abortSMSVerificationUnavailable(c)
					return
				}
				if count == 1 {
					if err := common.RDB.Expire(context.Background(), redisKey, time.Duration(SMSVerificationDuration)*time.Second).Err(); err != nil {
						abortSMSVerificationUnavailable(c)
						return
					}
				}
				if count > item.max {
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
			c.Next()
			return
		}

		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		for _, item := range keys {
			if !inMemoryRateLimiter.Request(item.name, int(item.max), SMSVerificationDuration) {
				c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": "发送过于频繁，请稍后再试"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
