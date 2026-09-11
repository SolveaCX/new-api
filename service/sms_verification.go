package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var ErrSMSProviderNotConfigured = errors.New("sms provider is not configured")

// SendSMSVerification sends a one-time code through TeleSign's messaging API.
// The mock mode is intentionally explicit and is only useful for local UI
// verification when no provider credentials are available.
func SendSMSVerification(ctx context.Context, phone, code string) error {
	normalizedPhone, err := common.NormalizePhoneNumber(phone)
	if err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}
	phone = normalizedPhone
	if common.TeleSignMockEnabled {
		common.SysLog("SMS verification mock")
		return nil
	}
	if !strings.HasPrefix(strings.TrimSpace(phone), "+86") {
		return sendITNIOSMSVerification(ctx, phone, code)
	}
	if strings.TrimSpace(common.TeleSignCustomerID) == "" || strings.TrimSpace(common.TeleSignAPIKey) == "" {
		return ErrSMSProviderNotConfigured
	}

	originator := strings.TrimSpace(common.TeleSignOriginator)
	if originator == "" {
		originator = "Flatkey"
	}
	form := url.Values{}
	form.Set("phone_number", phone)
	form.Set("message", fmt.Sprintf("Your %s verification code is %s.", originator, code))
	form.Set("message_type", "OTP")
	form.Set("sender_id", originator)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, common.TeleSignAPIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create SMS request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(common.TeleSignCustomerID, common.TeleSignAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send SMS request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("SMS provider returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func sendITNIOSMSVerification(ctx context.Context, phone, code string) error {
	if strings.TrimSpace(common.ITNIOAPIURL) == "" ||
		strings.TrimSpace(common.ITNIOAPIKey) == "" ||
		strings.TrimSpace(common.ITNIOAPISecret) == "" ||
		strings.TrimSpace(common.ITNIOAppID) == "" {
		return ErrSMSProviderNotConfigured
	}
	senderID := strings.TrimSpace(common.ITNIOSenderID)
	if senderID == "" {
		senderID = "Flatkey"
	}
	numbers := strings.TrimPrefix(strings.TrimSpace(phone), "+")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	digest := md5.Sum([]byte(common.ITNIOAPIKey + common.ITNIOAPISecret + timestamp))
	body, err := common.Marshal(map[string]any{
		"appId":       common.ITNIOAppID,
		"numbers":     numbers,
		"content":     fmt.Sprintf("Your %s verification code is %s.", senderID, code),
		"senderId":    senderID,
		"trackClicks": 0,
	})
	if err != nil {
		return fmt.Errorf("marshal ITNIO SMS request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, common.ITNIOAPIURL, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create ITNIO SMS request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("Sign", hex.EncodeToString(digest[:]))
	request.Header.Set("Timestamp", timestamp)
	request.Header.Set("Api-Key", common.ITNIOAPIKey)

	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("send ITNIO SMS request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("ITNIO returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := common.DecodeJson(response.Body, &result); err != nil {
		return fmt.Errorf("decode ITNIO response: %w", err)
	}
	if result.Status != "0" {
		return fmt.Errorf("ITNIO returned status %q", result.Status)
	}
	return nil
}
