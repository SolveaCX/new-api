package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RegistrationDomainState struct {
	Domain        string `json:"domain" gorm:"type:varchar(253);primaryKey"`
	ActiveBlockID int    `json:"active_block_id" gorm:"default:0;index"`
	CountingSince int64  `json:"counting_since" gorm:"default:0"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type RegistrationDomainBlock struct {
	Id                int    `json:"id"`
	Domain            string `json:"domain" gorm:"type:varchar(253);index"`
	WindowHours       int    `json:"window_hours"`
	Threshold         int    `json:"threshold"`
	ObservedCount     int    `json:"observed_count"`
	WindowStartedAt   int64  `json:"window_started_at"`
	BlockedAt         int64  `json:"blocked_at" gorm:"index"`
	ReleasedAt        int64  `json:"released_at" gorm:"default:0;index"`
	ReleasedBy        int    `json:"released_by" gorm:"default:0"`
	RestoreUsers      bool   `json:"restore_users" gorm:"default:false"`
	AffectedUserCount int64  `json:"affected_user_count" gorm:"-"`
}

type RegistrationDomainBlockUser struct {
	Id          int   `json:"id"`
	BlockID     int   `json:"block_id" gorm:"uniqueIndex:idx_registration_block_user"`
	UserID      int   `json:"user_id" gorm:"uniqueIndex:idx_registration_block_user;index"`
	PriorStatus int   `json:"prior_status"`
	DisabledAt  int64 `json:"disabled_at"`
	RestoredAt  int64 `json:"restored_at" gorm:"default:0"`
	RestoredBy  int   `json:"restored_by" gorm:"default:0"`
}

type RegistrationDomainRiskPolicy struct {
	Enabled   bool
	Window    time.Duration
	Threshold int
	Now       int64
}

type RegistrationDomainRiskResult struct {
	Triggered bool
	BlockID   int
}

type RegistrationDomainReleaseResult struct {
	Block         RegistrationDomainBlock `json:"block"`
	RestoredUsers int64                   `json:"restored_users"`
}

func RegisterUserWithDomainRisk(user *User, inviterID int, policy RegistrationDomainRiskPolicy, afterCreate func(*gorm.DB) error) (RegistrationDomainRiskResult, error) {
	if user.EmailDomain == "" && user.Email != "" {
		domain, err := common.NormalizeEmailDomain(user.Email)
		if err != nil {
			return RegistrationDomainRiskResult{}, err
		}
		user.EmailDomain = domain
	}
	if !policy.Enabled || user.EmailDomain == "" {
		return RegistrationDomainRiskResult{}, insertRegisteredUser(user, inviterID, afterCreate)
	}
	if policy.Now == 0 {
		policy.Now = time.Now().Unix()
	}
	if policy.Window <= 0 || policy.Threshold < 2 {
		return RegistrationDomainRiskResult{}, errors.New("invalid registration domain risk policy")
	}
	domain := strings.ToLower(strings.TrimSpace(user.EmailDomain))
	stateSeed := RegistrationDomainState{Domain: domain}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&stateSeed).Error; err != nil {
		return RegistrationDomainRiskResult{}, err
	}
	result := RegistrationDomainRiskResult{}
	var disabledIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RegistrationDomainState{}).Where("domain = ?", domain).
			UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; err != nil {
			return err
		}
		var state RegistrationDomainState
		if err := tx.Where("domain = ?", domain).First(&state).Error; err != nil {
			return err
		}
		if state.ActiveBlockID != 0 {
			result.BlockID = state.ActiveBlockID
			return nil
		}
		cutoff := policy.Now - int64(policy.Window/time.Second)
		if state.CountingSince > cutoff {
			cutoff = state.CountingSince
		}
		var count int64
		if err := usersForEmailDomain(tx.Model(&User{}), domain).
			Where("created_at >= ?", cutoff).Count(&count).Error; err != nil {
			return err
		}
		if count+1 < int64(policy.Threshold) {
			return insertRegisteredUserWithTx(tx, user, inviterID, afterCreate)
		}
		block := RegistrationDomainBlock{
			Domain: domain, WindowHours: int(policy.Window / time.Hour), Threshold: policy.Threshold,
			ObservedCount: int(count + 1), WindowStartedAt: cutoff, BlockedAt: policy.Now,
		}
		if err := tx.Create(&block).Error; err != nil {
			return err
		}
		var users []User
		if err := usersForEmailDomain(tx.Where("status = ?", common.UserStatusEnabled), domain).Find(&users).Error; err != nil {
			return err
		}
		if len(users) > 0 {
			affected := make([]RegistrationDomainBlockUser, 0, len(users))
			ids := make([]int, 0, len(users))
			for _, existing := range users {
				ids = append(ids, existing.Id)
				affected = append(affected, RegistrationDomainBlockUser{BlockID: block.Id, UserID: existing.Id, PriorStatus: existing.Status, DisabledAt: policy.Now})
			}
			disabledIDs = append(disabledIDs, ids...)
			if err := tx.Create(&affected).Error; err != nil {
				return err
			}
			if err := tx.Model(&User{}).Where("id IN ? AND status = ?", ids, common.UserStatusEnabled).
				Update("status", common.UserStatusDisabled).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&RegistrationDomainState{}).Where("domain = ?", domain).
			Update("active_block_id", block.Id).Error; err != nil {
			return err
		}
		result.Triggered = true
		result.BlockID = block.Id
		return nil
	})
	if err != nil {
		return RegistrationDomainRiskResult{}, err
	}
	for _, id := range disabledIDs {
		_ = InvalidateUserCache(id)
	}
	if result.BlockID != 0 {
		return result, ErrRegistrationDomainBlocked
	}
	return result, nil
}

func insertRegisteredUser(user *User, inviterID int, afterCreate func(*gorm.DB) error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return insertRegisteredUserWithTx(tx, user, inviterID, afterCreate)
	})
}

func insertRegisteredUserWithTx(tx *gorm.DB, user *User, inviterID int, afterCreate func(*gorm.DB) error) error {
	if err := user.InsertWithTx(tx, inviterID); err != nil {
		return err
	}
	if afterCreate != nil {
		return afterCreate(tx)
	}
	return nil
}

func usersForEmailDomain(db *gorm.DB, domain string) *gorm.DB {
	return db.Where("email_domain = ? OR (email_domain = '' AND LOWER(email) LIKE ?)", domain, "%@"+domain)
}

func IsRegistrationDomainBlocked(domain string) (bool, error) {
	var state RegistrationDomainState
	err := DB.Where("domain = ?", strings.ToLower(strings.TrimSpace(domain))).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return state.ActiveBlockID != 0, err
}

func ReleaseRegistrationDomainBlock(blockID int, adminID int, restoreUsers bool, releasedAt int64) (RegistrationDomainReleaseResult, error) {
	result := RegistrationDomainReleaseResult{}
	var restoredIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var block RegistrationDomainBlock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&block, blockID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRegistrationDomainBlockNotFound
			}
			return err
		}
		result.Block = block
		if block.ReleasedAt != 0 {
			return nil
		}
		if restoreUsers {
			var affected []RegistrationDomainBlockUser
			if err := tx.Where("block_id = ? AND restored_at = 0", blockID).Find(&affected).Error; err != nil {
				return err
			}
			for _, item := range affected {
				update := tx.Model(&User{}).Where("id = ? AND status = ?", item.UserID, common.UserStatusDisabled).
					Update("status", item.PriorStatus)
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected == 1 {
					result.RestoredUsers++
					restoredIDs = append(restoredIDs, item.UserID)
				}
				if err := tx.Model(&RegistrationDomainBlockUser{}).Where("id = ?", item.Id).
					Updates(map[string]any{"restored_at": releasedAt, "restored_by": adminID}).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&RegistrationDomainBlock{}).Where("id = ? AND released_at = 0", blockID).
			Updates(map[string]any{"released_at": releasedAt, "released_by": adminID, "restore_users": restoreUsers}).Error; err != nil {
			return err
		}
		if err := tx.Model(&RegistrationDomainState{}).Where("domain = ? AND active_block_id = ?", block.Domain, blockID).
			Updates(map[string]any{"active_block_id": 0, "counting_since": releasedAt}).Error; err != nil {
			return err
		}
		result.Block.ReleasedAt = releasedAt
		result.Block.ReleasedBy = adminID
		result.Block.RestoreUsers = restoreUsers
		return nil
	})
	if err != nil {
		return RegistrationDomainReleaseResult{}, err
	}
	for _, id := range restoredIDs {
		_ = InvalidateUserCache(id)
	}
	return result, nil
}
