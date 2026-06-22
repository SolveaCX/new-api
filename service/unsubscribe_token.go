package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

// unsubscribeSecret 返回 HMAC 签名密钥(复用全局 session secret)。
func unsubscribeSecret() []byte {
	return []byte(common.SessionSecret)
}

func unsubscribeSign(userId int) string {
	mac := hmac.New(sha256.New, unsubscribeSecret())
	mac.Write([]byte("unsub:" + strconv.Itoa(userId)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// GenerateUnsubscribeToken 为用户生成无状态、永久有效的退订 token。
func GenerateUnsubscribeToken(userId int) string {
	return unsubscribeSign(userId)
}

// VerifyUnsubscribeToken 校验退订 token。返回 (userId, 是否合法)。
func VerifyUnsubscribeToken(userId int, token string) (int, bool) {
	if token == "" {
		return 0, false
	}
	expected := unsubscribeSign(userId)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1 {
		return userId, true
	}
	return 0, false
}

// BuildUnsubscribeLink 拼出完整退订链接(origin 用 ServerAddress)。
func BuildUnsubscribeLink(serverAddress string, userId int) string {
	return fmt.Sprintf("%s/api/email/unsubscribe?uid=%d&token=%s",
		serverAddress, userId, GenerateUnsubscribeToken(userId))
}
