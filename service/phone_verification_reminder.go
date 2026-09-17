package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// PhoneVerificationReminderWindow is how long one served reminder silences
// further reminders for the same user, on every node.
const PhoneVerificationReminderWindow = 24 * time.Hour

var phoneVerificationReminderNow = time.Now

// phoneVerificationReminderMemory is the process-local fallback used when
// Redis is disabled. It only protects a single node; multi-node deployments
// run Redis and never reach it.
var phoneVerificationReminderMemory = struct {
	sync.Mutex
	claims map[int]time.Time
}{claims: map[int]time.Time{}}

func phoneVerificationReminderKey(userId int) string {
	return fmt.Sprintf("phone_reminder:api:%d", userId)
}

// ClaimPhoneVerificationReminder atomically claims the user's reminder slot
// for the next PhoneVerificationReminderWindow. It returns true for exactly
// one caller per user per window. Any Redis failure is returned as an error
// and the caller treats it as "not claimed", so a broken Redis can never turn
// every request into a reminder.
func ClaimPhoneVerificationReminder(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("phone verification reminder: invalid user id")
	}
	if common.RedisEnabled && common.RDB != nil {
		claimed, err := common.RDB.SetNX(context.Background(), phoneVerificationReminderKey(userId), "1", PhoneVerificationReminderWindow).Result()
		if err != nil {
			return false, fmt.Errorf("phone verification reminder: claim failed: %w", err)
		}
		return claimed, nil
	}

	now := phoneVerificationReminderNow()
	phoneVerificationReminderMemory.Lock()
	defer phoneVerificationReminderMemory.Unlock()
	for id, until := range phoneVerificationReminderMemory.claims {
		if !until.After(now) {
			delete(phoneVerificationReminderMemory.claims, id)
		}
	}
	if until, held := phoneVerificationReminderMemory.claims[userId]; held && until.After(now) {
		return false, nil
	}
	phoneVerificationReminderMemory.claims[userId] = now.Add(PhoneVerificationReminderWindow)
	return true, nil
}
