package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CustomerReferralOutboxPending    = "pending"
	CustomerReferralOutboxDelivering = "delivering"
	CustomerReferralOutboxDelivered  = "delivered"
	CustomerReferralOutboxDead       = "dead"
	customerReferralTimeLayout       = "2006-01-02T15:04:05.000Z07:00"
)

// CustomerReferralOutbox is the durable hand-off between registration and the
// Fluere callback. Payload is intentionally stored as the exact JSON string
// that will be sent so retries produce the same HMAC input and event body.
type CustomerReferralOutbox struct {
	Id            int64  `json:"id"`
	EventId       string `json:"event_id" gorm:"type:varchar(255);uniqueIndex"`
	UserId        int    `json:"user_id" gorm:"index"`
	Payload       string `json:"payload" gorm:"type:text"`
	Status        string `json:"status" gorm:"type:varchar(24);index;default:'pending'"`
	Attempts      int    `json:"attempts" gorm:"default:0"`
	NextAttemptAt int64  `json:"next_attempt_at" gorm:"index;default:0"`
	ClaimedAt     int64  `json:"claimed_at" gorm:"default:0"`
	DeliveredAt   int64  `json:"delivered_at" gorm:"default:0"`
	LastError     string `json:"last_error" gorm:"type:varchar(512);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type customerReferralPayload struct {
	SchemaVersion  string                   `json:"schema_version"`
	EventId        string                   `json:"event_id"`
	InviteCode     string                   `json:"invite_code"`
	SourcePlatform string                   `json:"source_platform"`
	Customer       customerReferralCustomer `json:"customer"`
	CreatedAt      string                   `json:"created_at"`
}

type customerReferralCustomer struct {
	CustomerId  string `json:"customer_id"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// ErrCustomerReferralCreatedAtMissing indicates that the user row cannot
// satisfy the callback contract because users.created_at is empty.
var ErrCustomerReferralCreatedAtMissing = errors.New("customer referral customer created_at is missing")

// EnqueueCustomerReferralInTx snapshots the transient invite fields into the
// same transaction as user creation. It is a no-op for non-ACTIVE users or
// registrations without a valid invite.
func EnqueueCustomerReferralInTx(tx *gorm.DB, user *User) error {
	if tx == nil || user == nil || user.Id <= 0 || user.Status != common.UserStatusEnabled {
		return nil
	}
	inviteCode := user.CustomerReferralInviteCode
	sourcePlatform := user.CustomerReferralSourcePlatform
	if strings.TrimSpace(inviteCode) == "" || strings.TrimSpace(sourcePlatform) == "" {
		return nil
	}
	if user.CreatedAt <= 0 {
		common.SysError(fmt.Sprintf("customer referral contract error: users.created_at is missing for user_id=%d", user.Id))
		return ErrCustomerReferralCreatedAtMissing
	}
	customerCreatedAt := time.Unix(user.CreatedAt, 0).UTC()
	eventCreatedAt := time.Now().UTC()
	eventID := fmt.Sprintf("flatkey-customer-created-%d", user.Id)
	payload := customerReferralPayload{
		SchemaVersion:  "flatkey-customer-referral-v1",
		EventId:        eventID,
		InviteCode:     inviteCode,
		SourcePlatform: sourcePlatform,
		Customer: customerReferralCustomer{
			CustomerId:  strconv.Itoa(user.Id),
			DisplayName: user.DisplayName,
			Status:      "ACTIVE",
			CreatedAt:   customerCreatedAt.Format(customerReferralTimeLayout),
		},
		CreatedAt: eventCreatedAt.Format(customerReferralTimeLayout),
	}
	raw, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	event := &CustomerReferralOutbox{
		EventId:       eventID,
		UserId:        user.Id,
		Payload:       string(raw),
		Status:        CustomerReferralOutboxPending,
		NextAttemptAt: common.GetTimestamp(),
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(event).Error
}

// ClaimCustomerReferralOutbox leases ready events to this process. The
// conditional update is the cross-node arbitration mechanism; no in-memory
// lock is relied upon for correctness.
func ClaimCustomerReferralOutbox(limit int, now int64) ([]CustomerReferralOutbox, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	staleBefore := now - 10*60
	if err := DB.Model(&CustomerReferralOutbox{}).
		Where("status = ? AND claimed_at > 0 AND claimed_at < ?", CustomerReferralOutboxDelivering, staleBefore).
		Updates(map[string]any{
			"status": CustomerReferralOutboxPending, "claimed_at": 0, "next_attempt_at": now,
		}).Error; err != nil {
		return nil, err
	}
	var candidates []CustomerReferralOutbox
	if err := DB.Where("status = ? AND next_attempt_at <= ?", CustomerReferralOutboxPending, now).
		Order("created_at asc").Limit(limit * 2).Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]CustomerReferralOutbox, 0, limit)
	for _, candidate := range candidates {
		result := DB.Model(&CustomerReferralOutbox{}).
			Where("id = ? AND status = ? AND next_attempt_at <= ?", candidate.Id, CustomerReferralOutboxPending, now).
			Updates(map[string]any{
				"status":     CustomerReferralOutboxDelivering,
				"claimed_at": now,
				"attempts":   gorm.Expr("attempts + 1"),
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		candidate.Status = CustomerReferralOutboxDelivering
		candidate.ClaimedAt = now
		candidate.Attempts++
		claimed = append(claimed, candidate)
		if len(claimed) >= limit {
			break
		}
	}
	return claimed, nil
}

func CompleteCustomerReferralOutbox(id int64, claimedAt int64, now int64) error {
	result := DB.Model(&CustomerReferralOutbox{}).
		Where("id = ? AND status = ? AND claimed_at = ?", id, CustomerReferralOutboxDelivering, claimedAt).
		Updates(map[string]any{
			"status": CustomerReferralOutboxDelivered, "delivered_at": now,
			"claimed_at": 0, "last_error": "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func FailCustomerReferralOutbox(id int64, claimedAt int64, attempts int, message string, now int64, retryable bool, maxAttempts int) error {
	if !retryable {
		return updateCustomerReferralOutboxFailure(id, claimedAt, CustomerReferralOutboxDead, 0, message)
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if attempts >= maxAttempts {
		return updateCustomerReferralOutboxFailure(id, claimedAt, CustomerReferralOutboxDead, 0, message)
	}
	delays := []int64{1, 5, 30, 120, 600}
	index := attempts - 1
	if index < 0 {
		index = 0
	}
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return updateCustomerReferralOutboxFailure(id, claimedAt, CustomerReferralOutboxPending, now+delays[index], message)
}

func updateCustomerReferralOutboxFailure(id int64, claimedAt int64, status string, nextAttemptAt int64, message string) error {
	message = truncateCustomerReferralError(message, 512)
	result := DB.Model(&CustomerReferralOutbox{}).
		Where("id = ? AND status = ? AND claimed_at = ?", id, CustomerReferralOutboxDelivering, claimedAt).
		Updates(map[string]any{
			"status": status, "next_attempt_at": nextAttemptAt,
			"claimed_at": 0, "last_error": message,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func truncateCustomerReferralError(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
