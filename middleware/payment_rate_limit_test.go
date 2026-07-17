package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func paymentTestRouter(userId int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	// Force the in-memory limiter: RedisEnabled defaults to true and RDB is
	// nil until InitRedisClient runs, which tests never do.
	common.RedisEnabled = false
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userId != 0 {
			c.Set("id", userId)
		}
	})
	r.POST("/pay", PaymentRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestPaymentRateLimitBlocksAfterLimit(t *testing.T) {
	origNum, origDur := common.PaymentRateLimitNum, common.PaymentRateLimitDuration
	common.PaymentRateLimitNum = 3
	common.PaymentRateLimitDuration = 60
	defer func() {
		common.PaymentRateLimitNum, common.PaymentRateLimitDuration = origNum, origDur
	}()

	r := paymentTestRouter(424241)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d", w.Code)
	}
}

func TestPaymentRateLimitIsPerUser(t *testing.T) {
	origNum, origDur := common.PaymentRateLimitNum, common.PaymentRateLimitDuration
	common.PaymentRateLimitNum = 1
	common.PaymentRateLimitDuration = 60
	defer func() {
		common.PaymentRateLimitNum, common.PaymentRateLimitDuration = origNum, origDur
	}()

	userA := paymentTestRouter(424251)
	userB := paymentTestRouter(424252)

	w := httptest.NewRecorder()
	userA.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("user A first request: expected 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	userA.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("user A second request: expected 429, got %d", w.Code)
	}
	// A hitting the limit must not affect B.
	w = httptest.NewRecorder()
	userB.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("user B first request: expected 200, got %d", w.Code)
	}
}

func TestPaymentRateLimitRejectsUnauthenticated(t *testing.T) {
	r := paymentTestRouter(0)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without user id, got %d", w.Code)
	}
}
