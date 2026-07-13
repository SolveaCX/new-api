package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

const migrateLegacyAffQuotaSQL = `
UPDATE users
SET quota = quota + aff_quota, aff_quota = 0
WHERE aff_quota > 0 AND deleted_at IS NULL`

const legacyAffQuotaMigrationBatchSize = 500

func MigrateLegacyAffQuotaToQuota() error {
	lastUserId := 0
	for {
		var users []User
		if err := DB.Model(&User{}).
			Select("id").
			Where("aff_quota > ? AND id > ?", 0, lastUserId).
			Order("id ASC").
			Limit(legacyAffQuotaMigrationBatchSize).
			Find(&users).Error; err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}

		userIds := make([]int, 0, len(users))
		for _, user := range users {
			userIds = append(userIds, user.Id)
		}
		if err := migrateLegacyAffQuotaBatch(userIds); err != nil {
			return err
		}
		lastUserId = users[len(users)-1].Id
	}
}

func migrateLegacyAffQuotaBatch(userIds []int) error {
	if len(userIds) == 0 {
		return nil
	}
	if err := DB.Exec(migrateLegacyAffQuotaSQL+" AND id IN ?", userIds).Error; err != nil {
		return err
	}
	for _, userId := range userIds {
		if err := InvalidateUserCache(userId); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate user %d cache after legacy invite reward migration: %v", userId, err))
		}
	}
	return nil
}

func MigrateUserLegacyAffQuotaToQuota(userId int) error {
	if userId <= 0 {
		return errors.New("user id must be positive")
	}

	result := DB.Exec(migrateLegacyAffQuotaSQL+" AND id = ?", userId)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user %d cache after legacy invite reward migration: %v", userId, err))
	}
	return nil
}
