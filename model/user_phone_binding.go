package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPhoneAlreadyTaken = errors.New("phone number already registered")

// UserPhoneBinding is the cross-node uniqueness ledger for verified phone
// numbers. The phone number is the primary key so concurrent registrations and
// bindings are serialized by the database rather than by a process-local lock.
type UserPhoneBinding struct {
	PhoneNumber string `json:"phone_number" gorm:"type:varchar(32);column:phone_number;primaryKey"`
	UserID      int    `json:"user_id" gorm:"column:user_id;uniqueIndex"`
	VerifiedAt  int64  `json:"verified_at" gorm:"column:verified_at;index"`
}

func normalizePhoneForBinding(phone string) (string, error) {
	return common.NormalizePhoneNumber(phone)
}

// IsPhoneAlreadyTaken checks the uniqueness ledger and also sees legacy user
// rows so deployments remain safe during the first migration rollout.
func IsPhoneAlreadyTaken(phone string) (bool, error) {
	normalized, err := normalizePhoneForBinding(phone)
	if err != nil {
		return false, nil
	}
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	var binding UserPhoneBinding
	err = DB.Unscoped().Where("phone_number = ?", normalized).First(&binding).Error
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	var count int64
	if err := DB.Unscoped().Model(&User{}).Where("phone_number = ?", normalized).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func reserveUserPhoneTx(tx *gorm.DB, phone string, userID int, verifiedAt int64) error {
	normalized, err := normalizePhoneForBinding(phone)
	if err != nil {
		return err
	}
	var binding UserPhoneBinding
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("phone_number = ?", normalized).First(&binding).Error
	if err == nil {
		if binding.UserID != userID {
			return ErrPhoneAlreadyTaken
		}
		return tx.Model(&binding).Updates(map[string]any{"verified_at": verifiedAt}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&UserPhoneBinding{PhoneNumber: normalized, UserID: userID, VerifiedAt: verifiedAt}).Error
}

// ReserveUserPhone reserves a normalized phone number in a transaction. It is
// exported for registration/binding flows and focused model tests.
func ReserveUserPhone(phone string, userID int, verifiedAt int64) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return reserveUserPhoneTx(tx, phone, userID, verifiedAt)
	})
}

// BindUserPhone atomically claims the phone and updates the user row.
func BindUserPhone(userID int, phone string, verifiedAt int64) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	normalized, err := normalizePhoneForBinding(phone)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := reserveUserPhoneTx(tx, normalized, userID, verifiedAt); err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
			"phone_number":      normalized,
			"phone_verified_at": verifiedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func backfillUserPhoneBindings(db *gorm.DB) error {
	var users []User
	if err := db.Unscoped().Where("phone_number <> ''").Find(&users).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			if err := reserveUserPhoneTx(tx, user.PhoneNumber, user.Id, user.PhoneVerifiedAt); err != nil {
				return err
			}
		}
		return nil
	})
}
