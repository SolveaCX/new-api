package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var ErrSMSProviderNotConfigured = errors.New("sms provider is not configured")

// SendSMSVerification sends a one-time code through TeleSign's messaging API.
// The mock mode is intentionally explicit and is only useful for local UI
// verification when no provider credentials are available.
func SendSMSVerification(ctx context.Context, phone, code string) error {
	if common.TeleSignMockEnabled {
		common.SysLog(fmt.Sprintf("SMS verification mock: phone=%s code=%s", phone, code))
		return nil
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
