package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	McpOAuthGrantStatusActive  = "active"
	McpOAuthGrantStatusRevoked = "revoked"

	McpOAuthRefreshTokenStatusActive  = "active"
	McpOAuthRefreshTokenStatusRotated = "rotated"
	McpOAuthRefreshTokenStatusRevoked = "revoked"
)

var (
	ErrMcpOAuthGrantRevoked       = errors.New("mcp oauth grant revoked")
	ErrMcpOAuthCredentialConsumed = errors.New("mcp oauth credential consumed")
	ErrMcpOAuthCredentialExpired  = errors.New("mcp oauth credential expired")
	ErrMcpOAuthRefreshReplay      = errors.New("mcp oauth refresh token replay")
)

type McpOAuthClient struct {
	Id           int    `json:"id"`
	PublicID     string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	Name         string `json:"name" gorm:"type:varchar(128);not null"`
	RedirectURIs string `json:"redirect_uris" gorm:"type:text"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64  `json:"updated_time" gorm:"bigint"`
	DisabledAt   int64  `json:"disabled_at" gorm:"bigint;default:0;index"`
}

func (McpOAuthClient) TableName() string {
	return "mcp_oauth_clients"
}

type McpOAuthGrant struct {
	Id               int    `json:"id"`
	PublicID         string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	ClientID         string `json:"client_id" gorm:"type:varchar(64);index"`
	UserID           int    `json:"user_id" gorm:"index"`
	DedicatedTokenId *int   `json:"dedicated_token_id" gorm:"uniqueIndex"`
	Status           string `json:"status" gorm:"type:varchar(16);index;default:'active'"`
	Scope            string `json:"scope" gorm:"type:varchar(1024);default:''"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime      int64  `json:"updated_time" gorm:"bigint"`
	RevokedAt        int64  `json:"revoked_at" gorm:"bigint;default:0;index"`
}

func (McpOAuthGrant) TableName() string {
	return "mcp_oauth_grants"
}

type McpOAuthAuthorizationCode struct {
	Id              int    `json:"id"`
	PublicID        string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	GrantPublicID   string `json:"grant_public_id" gorm:"type:varchar(64);index"`
	CodeHash        string `json:"-" gorm:"type:char(64);uniqueIndex"`
	RedirectURI     string `json:"redirect_uri" gorm:"type:varchar(2048);not null"`
	Scope           string `json:"scope" gorm:"type:varchar(1024);default:''"`
	CodeChallenge   string `json:"code_challenge" gorm:"type:varchar(128);default:''"`
	ChallengeMethod string `json:"challenge_method" gorm:"type:varchar(16);default:''"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;index"`
	ConsumedAt      int64  `json:"consumed_at" gorm:"bigint;default:0;index"`
}

func (McpOAuthAuthorizationCode) TableName() string {
	return "mcp_oauth_authorization_codes"
}

type McpOAuthRefreshToken struct {
	Id               int    `json:"id"`
	PublicID         string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	GrantPublicID    string `json:"grant_public_id" gorm:"type:varchar(64);index"`
	FamilyID         string `json:"family_id" gorm:"type:varchar(64);index"`
	TokenHash        string `json:"-" gorm:"type:char(64);uniqueIndex"`
	Status           string `json:"status" gorm:"type:varchar(16);index;default:'active'"`
	PreviousTokenId  int    `json:"previous_token_id" gorm:"index;default:0"`
	RotatedToTokenId int    `json:"rotated_to_token_id" gorm:"index;default:0"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint"`
	ExpiresAt        int64  `json:"expires_at" gorm:"bigint;index"`
	RotatedAt        int64  `json:"rotated_at" gorm:"bigint;default:0"`
	RevokedAt        int64  `json:"revoked_at" gorm:"bigint;default:0;index"`
	ReplayDetectedAt int64  `json:"replay_detected_at" gorm:"bigint;default:0"`
}

func (McpOAuthRefreshToken) TableName() string {
	return "mcp_oauth_refresh_tokens"
}

func HashMcpOAuthCredential(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func ProvisionMcpOAuthGrantDedicatedToken(grantPublicID string, token Token, now int64) (*Token, bool, error) {
	if grantPublicID == "" {
		return nil, false, gorm.ErrRecordNotFound
	}

	var result Token
	var created bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", grantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return ErrMcpOAuthGrantRevoked
		}
		if grant.DedicatedTokenId != nil && *grant.DedicatedTokenId > 0 {
			if err := tx.First(&result, *grant.DedicatedTokenId).Error; err != nil {
				return err
			}
			return nil
		}

		var existing Token
		err := tx.Where("oauth_grant_id = ?", grant.PublicID).First(&existing).Error
		if err == nil {
			tokenID := existing.Id
			if err := tx.Model(&grant).Updates(map[string]any{
				"dedicated_token_id": &tokenID,
				"updated_time":       now,
			}).Error; err != nil {
				return err
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		token.UserId = grant.UserID
		token.Source = TokenSourceMcpOAuth
		token.OAuthGrantId = &grant.PublicID
		if token.Status == 0 {
			token.Status = common.TokenStatusEnabled
		}
		if token.ExpiredTime == 0 {
			token.ExpiredTime = -1
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		tokenID := token.Id
		if err := tx.Model(&grant).Updates(map[string]any{
			"dedicated_token_id": &tokenID,
			"updated_time":       now,
		}).Error; err != nil {
			return err
		}
		result = token
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	result.Key = ""
	return &result, created, nil
}

func ConsumeMcpOAuthAuthorizationCode(codeHash string, now int64) (*McpOAuthAuthorizationCode, bool, error) {
	if codeHash == "" {
		return nil, false, gorm.ErrRecordNotFound
	}
	var consumedCode McpOAuthAuthorizationCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&McpOAuthAuthorizationCode{}).
			Where("code_hash = ? AND consumed_at = 0 AND expires_at > ?", codeHash, now).
			Update("consumed_at", now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 1 {
			return tx.Where("code_hash = ?", codeHash).First(&consumedCode).Error
		}

		var stored McpOAuthAuthorizationCode
		if err := tx.Where("code_hash = ?", codeHash).First(&stored).Error; err != nil {
			return err
		}
		if stored.ConsumedAt != 0 {
			return ErrMcpOAuthCredentialConsumed
		}
		if stored.ExpiresAt <= now {
			return ErrMcpOAuthCredentialExpired
		}
		return ErrMcpOAuthCredentialConsumed
	})
	if err != nil {
		return nil, false, err
	}
	return &consumedCode, true, nil
}

func RotateMcpOAuthRefreshToken(tokenHash string, next McpOAuthRefreshToken, now int64) (*McpOAuthRefreshToken, bool, error) {
	if tokenHash == "" {
		return nil, false, gorm.ErrRecordNotFound
	}
	var nextStored McpOAuthRefreshToken
	var replayDetected bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current McpOAuthRefreshToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			First(&current).Error; err != nil {
			return err
		}
		if current.Status != McpOAuthRefreshTokenStatusActive || current.RevokedAt != 0 {
			if err := revokeMcpOAuthRefreshFamilyInTx(tx, current.FamilyID, now, true); err != nil {
				return err
			}
			replayDetected = true
			return nil
		}
		if current.ExpiresAt <= now {
			return ErrMcpOAuthCredentialExpired
		}

		update := tx.Model(&McpOAuthRefreshToken{}).
			Where("id = ? AND status = ? AND revoked_at = 0 AND expires_at > ?", current.Id, McpOAuthRefreshTokenStatusActive, now).
			Updates(map[string]any{
				"status":     McpOAuthRefreshTokenStatusRotated,
				"rotated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			if err := tx.First(&current, current.Id).Error; err != nil {
				return err
			}
			if current.Status != McpOAuthRefreshTokenStatusActive || current.RevokedAt != 0 {
				if err := revokeMcpOAuthRefreshFamilyInTx(tx, current.FamilyID, now, true); err != nil {
					return err
				}
				replayDetected = true
				return nil
			}
			return ErrMcpOAuthCredentialExpired
		}

		next.GrantPublicID = current.GrantPublicID
		if next.FamilyID == "" {
			next.FamilyID = current.FamilyID
		}
		if next.FamilyID == "" {
			next.FamilyID = current.PublicID
		}
		next.Status = McpOAuthRefreshTokenStatusActive
		next.PreviousTokenId = current.Id
		if err := tx.Create(&next).Error; err != nil {
			return err
		}
		if err := tx.Model(&McpOAuthRefreshToken{}).
			Where("id = ?", current.Id).
			Update("rotated_to_token_id", next.Id).Error; err != nil {
			return err
		}
		nextStored = next
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if replayDetected {
		return nil, false, ErrMcpOAuthRefreshReplay
	}
	return &nextStored, true, nil
}

func RevokeMcpOAuthGrant(grantPublicID string, now int64) (bool, error) {
	if grantPublicID == "" {
		return false, gorm.ErrRecordNotFound
	}
	var revoked bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", grantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return nil
		}
		update := tx.Model(&McpOAuthGrant{}).
			Where("id = ? AND status <> ? AND revoked_at = 0", grant.Id, McpOAuthGrantStatusRevoked).
			Updates(map[string]any{
				"status":       McpOAuthGrantStatusRevoked,
				"revoked_at":   now,
				"updated_time": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return nil
		}
		revoked = true
		if grant.DedicatedTokenId != nil && *grant.DedicatedTokenId > 0 {
			if err := tx.Model(&Token{}).
				Where("id = ? AND oauth_grant_id = ?", *grant.DedicatedTokenId, grant.PublicID).
				Update("status", common.TokenStatusDisabled).Error; err != nil {
				return err
			}
		}
		return revokeMcpOAuthGrantRefreshTokensInTx(tx, grant.PublicID, now)
	})
	return revoked, err
}

func revokeMcpOAuthGrantRefreshTokensInTx(tx *gorm.DB, grantPublicID string, now int64) error {
	if grantPublicID == "" {
		return nil
	}
	return tx.Model(&McpOAuthRefreshToken{}).
		Where("grant_public_id = ? AND status <> ?", grantPublicID, McpOAuthRefreshTokenStatusRevoked).
		Updates(map[string]any{
			"status":     McpOAuthRefreshTokenStatusRevoked,
			"revoked_at": now,
		}).Error
}

func revokeMcpOAuthRefreshFamilyInTx(tx *gorm.DB, familyID string, now int64, markReplay bool) error {
	if familyID == "" {
		return nil
	}
	updates := map[string]any{
		"status":     McpOAuthRefreshTokenStatusRevoked,
		"revoked_at": now,
	}
	if markReplay {
		updates["replay_detected_at"] = now
	}
	return tx.Model(&McpOAuthRefreshToken{}).
		Where("family_id = ? AND status <> ?", familyID, McpOAuthRefreshTokenStatusRevoked).
		Updates(updates).Error
}
