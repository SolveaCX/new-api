package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	customerReferralCallbackURL = "https://fluere.io/api/internal/flatkey/customer-created"
	customerReferralCallbackPath = "/api/internal/flatkey/customer-created"
	customerReferralMaxAttempts = 6 // initial delivery plus five retries
)

var customerReferralDeliveryOnce sync.Once

// CustomerInvite is the authenticated referral data carried by a Fluere invite.
// It is deliberately kept in memory only until the registration transaction
// snapshots it into the internal callback outbox.
type CustomerInvite struct {
	Code     string `json:"code"`
	Platform string `json:"platform"`
}

// DecodeCustomerInvite decrypts the versioned AES-256-GCM invite envelope.
// The key is read only on the server from FLATKEY_CUSTOMER_INVITE_ENCRYPTION_KEY.
func DecodeCustomerInvite(raw string) (CustomerInvite, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 8192 {
		return CustomerInvite{}, errors.New("empty customer invite")
	}
	keyHex := strings.TrimSpace(os.Getenv("FLATKEY_CUSTOMER_INVITE_ENCRYPTION_KEY"))
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return CustomerInvite{}, errors.New("customer invite encryption key is not a 32-byte hex key")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(sealed) < 1+12+16 {
		return CustomerInvite{}, errors.New("invalid customer invite encoding")
	}
	if sealed[0] != 1 {
		return CustomerInvite{}, errors.New("unsupported customer invite version")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return CustomerInvite{}, errors.New("failed to initialize customer invite cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return CustomerInvite{}, errors.New("failed to initialize customer invite gcm")
	}
	plaintext, err := gcm.Open(nil, sealed[1:13], sealed[13:], nil)
	if err != nil {
		return CustomerInvite{}, errors.New("customer invite authentication failed")
	}
	var invite CustomerInvite
	if err := common.Unmarshal(plaintext, &invite); err != nil {
		return CustomerInvite{}, errors.New("invalid customer invite payload")
	}
	if strings.TrimSpace(invite.Code) == "" || strings.TrimSpace(invite.Platform) == "" || len(invite.Code) > 512 || len(invite.Platform) > 64 {
		return CustomerInvite{}, errors.New("customer invite payload is incomplete")
	}
	return invite, nil
}

type customerReferralCallbackConfig struct {
	URL         string
	Token       string
	HMACSecret  string
}

func loadCustomerReferralCallbackConfig() customerReferralCallbackConfig {
	callbackURL := strings.TrimSpace(os.Getenv("FLATKEY_CUSTOMER_CALLBACK_URL"))
	if callbackURL == "" {
		callbackURL = customerReferralCallbackURL
	}
	return customerReferralCallbackConfig{
		URL:        strings.TrimRight(callbackURL, "/"),
		Token:      strings.TrimSpace(os.Getenv("FLATKEY_CUSTOMER_CALLBACK_TOKEN")),
		HMACSecret: os.Getenv("FLATKEY_CUSTOMER_CALLBACK_HMAC_SECRET"),
	}
}

func (config customerReferralCallbackConfig) valid() bool {
	return config.URL != "" && config.Token != "" && config.HMACSecret != ""
}

func signCustomerReferralPayload(timestamp int64, rawBody []byte, secret string) string {
	message := strconv.FormatInt(timestamp, 10) + "." + customerReferralCallbackPath + "." + string(rawBody)
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

type customerReferralHTTPError struct {
	status int
}

func (e *customerReferralHTTPError) Error() string {
	return fmt.Sprintf("customer referral callback returned HTTP %d", e.status)
}

func runCustomerReferralDeliveryOnce(config customerReferralCallbackConfig, client *http.Client) {
	now := common.GetTimestamp()
	events, err := model.ClaimCustomerReferralOutbox(50, now)
	if err != nil {
		logger.LogError(nil, "customer referral outbox claim failed: "+err.Error())
		return
	}
	for _, event := range events {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		deliveryErr := deliverCustomerReferralEventWithContext(ctx, config, client, event)
		cancel()
		if deliveryErr == nil {
			if err := model.CompleteCustomerReferralOutbox(event.Id, event.ClaimedAt, common.GetTimestamp()); err != nil {
				logger.LogError(nil, "customer referral outbox completion failed: "+err.Error())
			}
			continue
		}
		retryable := true
		var statusErr *customerReferralHTTPError
		if errors.As(deliveryErr, &statusErr) {
			retryable = statusErr.status >= 500 && statusErr.status <= 599
		}
		if err := model.FailCustomerReferralOutbox(event.Id, event.ClaimedAt, event.Attempts, deliveryErr.Error(), common.GetTimestamp(), retryable, customerReferralMaxAttempts); err != nil {
			logger.LogError(nil, "customer referral outbox retry scheduling failed: "+err.Error())
		}
	}
}

func deliverCustomerReferralEventWithContext(ctx context.Context, config customerReferralCallbackConfig, client *http.Client, event model.CustomerReferralOutbox) error {
	rawBody := []byte(event.Payload)
	timestamp := time.Now().UnixMilli()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.URL, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+config.Token)
	request.Header.Set("X-Request-Timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("X-Request-Signature", signCustomerReferralPayload(timestamp, rawBody, config.HMACSecret))
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusCreated || response.StatusCode == http.StatusOK {
		return nil
	}
	return &customerReferralHTTPError{status: response.StatusCode}
}

// StartCustomerReferralDeliveryTask runs only on master nodes. Conditional DB
// claims make delivery safe if multiple replicas briefly believe they are master.
func StartCustomerReferralDeliveryTask() {
	customerReferralDeliveryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		config := loadCustomerReferralCallbackConfig()
		if !config.valid() {
			logger.LogInfo(context.Background(), "customer referral delivery disabled: configuration incomplete")
			return
		}
		client := &http.Client{Timeout: 15 * time.Second}
		gopool.Go(func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			runCustomerReferralDeliveryOnce(config, client)
			for range ticker.C {
				runCustomerReferralDeliveryOnce(config, client)
			}
		})
	})
}
