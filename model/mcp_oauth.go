package model

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	McpOAuthGrantStatusPending = "pending"
	McpOAuthGrantStatusActive  = "active"
	McpOAuthGrantStatusFailed  = "failed"
	McpOAuthGrantStatusRevoked = "revoked"

	McpOAuthRefreshTokenStatusActive  = "active"
	McpOAuthRefreshTokenStatusRotated = "rotated"
	McpOAuthRefreshTokenStatusRevoked = "revoked"
)

var (
	ErrMcpOAuthGrantRevoked             = errors.New("mcp oauth grant revoked")
	ErrMcpOAuthCredentialConsumed       = errors.New("mcp oauth credential consumed")
	ErrMcpOAuthCredentialExpired        = errors.New("mcp oauth credential expired")
	ErrMcpOAuthRefreshReplay            = errors.New("mcp oauth refresh token replay")
	ErrMcpOAuthGrantUnavailable         = errors.New("mcp oauth grant unavailable")
	ErrMcpOAuthGrantTokenLinkCorrupt    = errors.New("mcp oauth grant token link corrupt")
	ErrMcpOAuthRefreshFamilyConflict    = errors.New("mcp oauth refresh family conflict")
	ErrMcpOAuthApprovalAlreadyProcessed = errors.New("mcp oauth approval already processed")
	ErrMcpOAuthCredentialMismatch       = errors.New("mcp oauth credential mismatch")
	ErrMcpOAuthPKCEMismatch             = errors.New("mcp oauth pkce mismatch")
)

type McpOAuthClient struct {
	Id           int    `json:"id"`
	PublicID     string `json:"public_id" gorm:"type:varchar(512);uniqueIndex"`
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
	Id                   int     `json:"id"`
	PublicID             string  `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	ClientID             string  `json:"client_id" gorm:"type:varchar(512);index"`
	UserID               int     `json:"user_id" gorm:"index"`
	AccountID            int     `json:"account_id" gorm:"index"`
	Resource             string  `json:"resource" gorm:"type:varchar(255);index;default:''"`
	DisplayName          string  `json:"display_name" gorm:"type:varchar(128);default:''"`
	DedicatedTokenId     *int    `json:"dedicated_token_id" gorm:"uniqueIndex"`
	RefreshTokenFamilyId *string `json:"refresh_token_family_id" gorm:"type:varchar(64);uniqueIndex"`
	Status               string  `json:"status" gorm:"type:varchar(16);index;default:'pending'"`
	Scope                string  `json:"scope" gorm:"type:varchar(1024);default:''"`
	CreatedTime          int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime          int64   `json:"updated_time" gorm:"bigint"`
	LastUsedAt           int64   `json:"last_used_at" gorm:"bigint;default:0;index"`
	RevokedAt            int64   `json:"revoked_at" gorm:"bigint;default:0;index"`
}

func (McpOAuthGrant) TableName() string {
	return "mcp_oauth_grants"
}

type McpOAuthAuthorizationCode struct {
	Id                  int     `json:"id"`
	PublicID            string  `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	GrantPublicID       string  `json:"grant_public_id" gorm:"type:varchar(64);index"`
	CodeHash            string  `json:"-" gorm:"type:char(64);uniqueIndex"`
	ApprovalFingerprint *string `json:"approval_fingerprint,omitempty" gorm:"type:char(64);uniqueIndex"`
	RedirectURI         string  `json:"redirect_uri" gorm:"type:varchar(2048);not null"`
	Scope               string  `json:"scope" gorm:"type:varchar(1024);default:''"`
	CodeChallenge       string  `json:"code_challenge" gorm:"type:varchar(128);default:''"`
	ChallengeMethod     string  `json:"challenge_method" gorm:"type:varchar(16);default:''"`
	CreatedTime         int64   `json:"created_time" gorm:"bigint"`
	ExpiresAt           int64   `json:"expires_at" gorm:"bigint;index"`
	ConsumedAt          int64   `json:"consumed_at" gorm:"bigint;default:0;index"`
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

func FingerprintMcpOAuthApproval(userID int, clientID, resource, redirectURI, scope, codeChallenge string) string {
	parts := []string{
		strconv.Itoa(userID),
		strings.TrimSpace(clientID),
		strings.TrimSpace(resource),
		strings.TrimSpace(redirectURI),
		normalizeMcpOAuthFingerprintScope(scope),
		strings.TrimSpace(codeChallenge),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeMcpOAuthFingerprintScope(scope string) string {
	fields := strings.Fields(scope)
	if len(fields) <= 1 {
		return strings.TrimSpace(scope)
	}
	seen := map[string]bool{}
	ordered := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		ordered = append(ordered, field)
	}
	return strings.Join(ordered, " ")
}

type McpOAuthApprovalCreateParams struct {
	GrantPublicID       string
	CodePublicID        string
	CodeSecretHash      string
	ApprovalFingerprint string
	ClientID            string
	UserID              int
	AccountID           int
	Resource            string
	DisplayName         string
	Scope               string
	RedirectURI         string
	CodeChallenge       string
	ChallengeMethod     string
	TokenKey            string
	TokenRemainQuota    int
	TokenUnlimitedQuota bool
	Now                 int64
	CodeExpiresAt       int64
}

func CreateMcpOAuthApproval(params McpOAuthApprovalCreateParams) (*McpOAuthGrant, *McpOAuthAuthorizationCode, error) {
	var grant McpOAuthGrant
	var code McpOAuthAuthorizationCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		grant = McpOAuthGrant{
			PublicID:    params.GrantPublicID,
			ClientID:    params.ClientID,
			UserID:      params.UserID,
			AccountID:   params.AccountID,
			Resource:    params.Resource,
			DisplayName: params.DisplayName,
			Status:      McpOAuthGrantStatusPending,
			Scope:       params.Scope,
			CreatedTime: params.Now,
			UpdatedTime: params.Now,
		}
		if err := tx.Create(&grant).Error; err != nil {
			return err
		}
		token := Token{
			UserId:         params.UserID,
			Name:           "MCP OAuth dedicated token",
			Key:            params.TokenKey,
			Status:         common.TokenStatusEnabled,
			CreatedTime:    params.Now,
			AccessedTime:   params.Now,
			ExpiredTime:    -1,
			RemainQuota:    params.TokenRemainQuota,
			UnlimitedQuota: params.TokenUnlimitedQuota,
			Group:          "default",
			Source:         TokenSourceMcpOAuth,
			OAuthGrantId:   &grant.PublicID,
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		tokenID := token.Id
		if err := tx.Model(&grant).Updates(map[string]any{
			"dedicated_token_id": &tokenID,
			"status":             McpOAuthGrantStatusActive,
			"updated_time":       params.Now,
		}).Error; err != nil {
			return err
		}
		grant.DedicatedTokenId = &tokenID
		grant.Status = McpOAuthGrantStatusActive
		var approvalFingerprint *string
		if strings.TrimSpace(params.ApprovalFingerprint) != "" {
			approvalFingerprint = &params.ApprovalFingerprint
		}
		code = McpOAuthAuthorizationCode{
			PublicID:            params.CodePublicID,
			GrantPublicID:       grant.PublicID,
			CodeHash:            params.CodeSecretHash,
			ApprovalFingerprint: approvalFingerprint,
			RedirectURI:         params.RedirectURI,
			Scope:               params.Scope,
			CodeChallenge:       params.CodeChallenge,
			ChallengeMethod:     params.ChallengeMethod,
			CreatedTime:         params.Now,
			ExpiresAt:           params.CodeExpiresAt,
		}
		if err := tx.Create(&code).Error; err != nil {
			if isMcpOAuthUniqueConstraintError(err) {
				return ErrMcpOAuthApprovalAlreadyProcessed
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &grant, &code, nil
}

func isMcpOAuthUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unique") || strings.Contains(text, "duplicate") || strings.Contains(text, "constraint")
}

func validateMcpOAuthPKCEInModel(verifier, challenge, method string) error {
	if method != "S256" {
		return ErrMcpOAuthPKCEMismatch
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		return ErrMcpOAuthPKCEMismatch
	}
	for _, r := range verifier {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' || r == '~') {
			return ErrMcpOAuthPKCEMismatch
		}
	}
	sum := sha256.Sum256([]byte(verifier))
	if challenge == "" || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
		return ErrMcpOAuthPKCEMismatch
	}
	return nil
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
		if grant.Status != McpOAuthGrantStatusPending && grant.Status != McpOAuthGrantStatusActive {
			return ErrMcpOAuthGrantUnavailable
		}
		if grant.DedicatedTokenId != nil && *grant.DedicatedTokenId > 0 {
			err := tx.Where("id = ? AND source = ? AND oauth_grant_id = ?", *grant.DedicatedTokenId, TokenSourceMcpOAuth, grant.PublicID).
				First(&result).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMcpOAuthGrantTokenLinkCorrupt
			}
			if err != nil {
				return err
			}
			return nil
		}

		var existing Token
		err := tx.Where("source = ? AND oauth_grant_id = ?", TokenSourceMcpOAuth, grant.PublicID).First(&existing).Error
		if err == nil {
			tokenID := existing.Id
			if err := tx.Model(&grant).Updates(map[string]any{
				"dedicated_token_id": &tokenID,
				"status":             McpOAuthGrantStatusActive,
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
			"status":             McpOAuthGrantStatusActive,
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
		var lookup McpOAuthAuthorizationCode
		if err := tx.Select("id", "grant_public_id", "consumed_at", "expires_at").
			Where("code_hash = ?", codeHash).
			First(&lookup).Error; err != nil {
			return err
		}
		if lookup.ConsumedAt != 0 {
			return ErrMcpOAuthCredentialConsumed
		}
		if lookup.ExpiresAt <= now {
			return ErrMcpOAuthCredentialExpired
		}
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", lookup.GrantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		var stored McpOAuthAuthorizationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND code_hash = ? AND grant_public_id = ?", lookup.Id, codeHash, grant.PublicID).
			First(&stored).Error; err != nil {
			return err
		}
		if stored.ConsumedAt != 0 {
			return ErrMcpOAuthCredentialConsumed
		}
		if stored.ExpiresAt <= now {
			return ErrMcpOAuthCredentialExpired
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return ErrMcpOAuthGrantRevoked
		}
		if grant.Status != McpOAuthGrantStatusActive {
			return ErrMcpOAuthGrantUnavailable
		}

		update := tx.Model(&McpOAuthAuthorizationCode{}).
			Where("id = ? AND consumed_at = 0 AND expires_at > ?", stored.Id, now).
			Update("consumed_at", now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 1 {
			stored.ConsumedAt = now
			consumedCode = stored
			return nil
		}

		if err := tx.Where("id = ?", stored.Id).First(&stored).Error; err != nil {
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

type ExchangeMcpOAuthAuthorizationCodeParams struct {
	CodeHash           string
	ClientID           string
	Resource           string
	RedirectURI        string
	CodeVerifier       string
	RefreshPublicID    string
	RefreshTokenHash   string
	RefreshTokenFamily string
	Now                int64
	RefreshExpiresAt   int64
}

type McpOAuthCodeExchange struct {
	Grant McpOAuthGrant
	Code  McpOAuthAuthorizationCode
}

func GetMcpOAuthAuthorizationCodeExchangeSnapshot(codeHash string) (*McpOAuthCodeExchange, error) {
	if codeHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var code McpOAuthAuthorizationCode
	if err := DB.Where("code_hash = ?", codeHash).First(&code).Error; err != nil {
		return nil, err
	}
	var grant McpOAuthGrant
	if err := DB.Where("public_id = ?", code.GrantPublicID).First(&grant).Error; err != nil {
		return nil, err
	}
	return &McpOAuthCodeExchange{Grant: grant, Code: code}, nil
}

func ExchangeMcpOAuthAuthorizationCode(params ExchangeMcpOAuthAuthorizationCodeParams) (*McpOAuthCodeExchange, *McpOAuthRefreshToken, error) {
	var exchanged McpOAuthCodeExchange
	var refresh McpOAuthRefreshToken
	err := DB.Transaction(func(tx *gorm.DB) error {
		var lookup McpOAuthAuthorizationCode
		if err := tx.Select("id", "grant_public_id").
			Where("code_hash = ?", params.CodeHash).
			First(&lookup).Error; err != nil {
			return err
		}
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", lookup.GrantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		var code McpOAuthAuthorizationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND code_hash = ? AND grant_public_id = ?", lookup.Id, params.CodeHash, grant.PublicID).
			First(&code).Error; err != nil {
			return err
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return ErrMcpOAuthGrantRevoked
		}
		if grant.Status != McpOAuthGrantStatusActive {
			return ErrMcpOAuthGrantUnavailable
		}
		if code.ConsumedAt != 0 {
			return ErrMcpOAuthCredentialConsumed
		}
		if code.ExpiresAt <= params.Now {
			return ErrMcpOAuthCredentialExpired
		}
		if grant.ClientID != params.ClientID || grant.Resource != params.Resource || code.RedirectURI != params.RedirectURI {
			return ErrMcpOAuthCredentialMismatch
		}
		if err := validateMcpOAuthPKCEInModel(params.CodeVerifier, code.CodeChallenge, code.ChallengeMethod); err != nil {
			return err
		}
		refresh = McpOAuthRefreshToken{
			PublicID:      params.RefreshPublicID,
			GrantPublicID: grant.PublicID,
			FamilyID:      params.RefreshTokenFamily,
			TokenHash:     params.RefreshTokenHash,
			Status:        McpOAuthRefreshTokenStatusActive,
			CreatedTime:   params.Now,
			ExpiresAt:     params.RefreshExpiresAt,
		}
		if err := tx.Create(&refresh).Error; err != nil {
			return err
		}
		update := tx.Model(&McpOAuthAuthorizationCode{}).
			Where("id = ? AND consumed_at = 0 AND expires_at > ?", code.Id, params.Now).
			Update("consumed_at", params.Now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrMcpOAuthCredentialConsumed
		}
		code.ConsumedAt = params.Now
		familyID := refresh.FamilyID
		if familyID == "" {
			familyID = refresh.PublicID
			if err := tx.Model(&refresh).Update("family_id", familyID).Error; err != nil {
				return err
			}
			refresh.FamilyID = familyID
		}
		if grant.RefreshTokenFamilyId != nil && *grant.RefreshTokenFamilyId != familyID {
			return ErrMcpOAuthRefreshFamilyConflict
		}
		if err := tx.Model(&grant).Updates(map[string]any{
			"refresh_token_family_id": &familyID,
			"last_used_at":            params.Now,
			"updated_time":            params.Now,
		}).Error; err != nil {
			return err
		}
		grant.RefreshTokenFamilyId = &familyID
		grant.LastUsedAt = params.Now
		grant.UpdatedTime = params.Now
		exchanged = McpOAuthCodeExchange{Grant: grant, Code: code}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &exchanged, &refresh, nil
}

type McpOAuthRefreshSnapshot struct {
	Grant   McpOAuthGrant
	Refresh McpOAuthRefreshToken
}

func GetMcpOAuthRefreshSnapshotByHash(tokenHash string) (*McpOAuthRefreshSnapshot, error) {
	if tokenHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var refresh McpOAuthRefreshToken
	if err := DB.Where("token_hash = ?", tokenHash).First(&refresh).Error; err != nil {
		return nil, err
	}
	var grant McpOAuthGrant
	if err := DB.Where("public_id = ?", refresh.GrantPublicID).First(&grant).Error; err != nil {
		return nil, err
	}
	return &McpOAuthRefreshSnapshot{Grant: grant, Refresh: refresh}, nil
}

func RotateMcpOAuthRefreshToken(tokenHash string, next McpOAuthRefreshToken, now int64) (*McpOAuthRefreshToken, bool, error) {
	if tokenHash == "" {
		return nil, false, gorm.ErrRecordNotFound
	}
	var nextStored McpOAuthRefreshToken
	var replayDetected bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var lookup McpOAuthRefreshToken
		if err := tx.Select("id", "grant_public_id").
			Where("token_hash = ?", tokenHash).
			First(&lookup).Error; err != nil {
			return err
		}
		var current McpOAuthRefreshToken
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", lookup.GrantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND token_hash = ? AND grant_public_id = ?", lookup.Id, tokenHash, grant.PublicID).
			First(&current).Error; err != nil {
			return err
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return ErrMcpOAuthGrantRevoked
		}
		if grant.Status != McpOAuthGrantStatusActive {
			return ErrMcpOAuthGrantUnavailable
		}
		effectiveFamilyID := current.FamilyID
		if effectiveFamilyID == "" {
			effectiveFamilyID = current.PublicID
		}
		if grant.RefreshTokenFamilyId != nil && *grant.RefreshTokenFamilyId != effectiveFamilyID {
			return ErrMcpOAuthRefreshFamilyConflict
		}
		if current.FamilyID == "" {
			if err := tx.Model(&current).Update("family_id", effectiveFamilyID).Error; err != nil {
				return err
			}
			current.FamilyID = effectiveFamilyID
		}
		if grant.RefreshTokenFamilyId == nil {
			if err := tx.Model(&grant).Updates(map[string]any{
				"refresh_token_family_id": &effectiveFamilyID,
				"last_used_at":            now,
				"updated_time":            now,
			}).Error; err != nil {
				return err
			}
			grant.RefreshTokenFamilyId = &effectiveFamilyID
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
		if grant.DedicatedTokenId != nil && *grant.DedicatedTokenId > 0 {
			var linked Token
			err := tx.Where("id = ? AND source = ? AND oauth_grant_id = ?", *grant.DedicatedTokenId, TokenSourceMcpOAuth, grant.PublicID).
				First(&linked).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMcpOAuthGrantTokenLinkCorrupt
			}
			if err != nil {
				return err
			}
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

type McpOAuthCredentialRevokeParams struct {
	CredentialHash string
	ClientID       string
	Now            int64
}

func RevokeMcpOAuthGrantByCredential(params McpOAuthCredentialRevokeParams) (bool, error) {
	if params.CredentialHash == "" {
		return false, nil
	}
	var grantPublicID string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var refresh McpOAuthRefreshToken
		err := tx.Select("grant_public_id").
			Where("token_hash = ?", params.CredentialHash).
			First(&refresh).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", refresh.GrantPublicID).
			First(&grant).Error; err != nil {
			return err
		}
		if params.ClientID != "" && grant.ClientID != params.ClientID {
			return nil
		}
		grantPublicID = grant.PublicID
		return nil
	})
	if err != nil || grantPublicID == "" {
		return false, err
	}
	return RevokeMcpOAuthGrant(grantPublicID, params.Now)
}

type McpOAuthConnectedApp struct {
	GrantPublicID string `json:"grant_public_id"`
	ClientID      string `json:"client_id"`
	DisplayName   string `json:"display_name"`
	Scopes        string `json:"scopes"`
	CreatedTime   int64  `json:"created_time"`
	LastUsedAt    int64  `json:"last_used_at"`
	Status        string `json:"status"`
}

func ListMcpOAuthConnectedApps(userID int) ([]McpOAuthConnectedApp, error) {
	var grants []McpOAuthGrant
	if err := DB.Where("user_id = ?", userID).
		Order("id desc").
		Find(&grants).Error; err != nil {
		return nil, err
	}
	apps := make([]McpOAuthConnectedApp, 0, len(grants))
	for _, grant := range grants {
		apps = append(apps, McpOAuthConnectedApp{
			GrantPublicID: grant.PublicID,
			ClientID:      grant.ClientID,
			DisplayName:   grant.DisplayName,
			Scopes:        grant.Scope,
			CreatedTime:   grant.CreatedTime,
			LastUsedAt:    grant.LastUsedAt,
			Status:        grant.Status,
		})
	}
	return apps, nil
}

func RevokeMcpOAuthConnectedApp(userID int, grantPublicID string, now int64) (bool, error) {
	if userID <= 0 || grantPublicID == "" {
		return false, nil
	}
	var owned bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var grant McpOAuthGrant
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("public_id", "user_id").
			Where("public_id = ?", grantPublicID).
			First(&grant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		owned = grant.UserID == userID
		return nil
	})
	if err != nil || !owned {
		return false, err
	}
	return RevokeMcpOAuthGrant(grantPublicID, now)
}

type McpOAuthDataToolClaims struct {
	Subject  string
	GrantID  string
	ClientID string
	Resource string
}

type McpOAuthDataToolIdentity struct {
	UserID        int    `json:"user_id"`
	GrantPublicID string `json:"grant_public_id"`
	ClientID      string `json:"client_id"`
	Resource      string `json:"resource"`
	Scopes        string `json:"scopes"`
	Token         Token  `json:"-"`
}

func ResolveMcpOAuthDataToolIdentity(claims McpOAuthDataToolClaims, now int64) (*McpOAuthDataToolIdentity, error) {
	var identity McpOAuthDataToolIdentity
	err := DB.Transaction(func(tx *gorm.DB) error {
		var grant McpOAuthGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", claims.GrantID).
			First(&grant).Error; err != nil {
			return err
		}
		if grant.Status == McpOAuthGrantStatusRevoked || grant.RevokedAt != 0 {
			return ErrMcpOAuthGrantRevoked
		}
		if grant.Status != McpOAuthGrantStatusActive {
			return ErrMcpOAuthGrantUnavailable
		}
		if claims.Subject != fmt.Sprintf("user-%d", grant.UserID) || claims.ClientID != grant.ClientID || claims.Resource != grant.Resource {
			return ErrMcpOAuthCredentialMismatch
		}
		if grant.DedicatedTokenId == nil || *grant.DedicatedTokenId <= 0 {
			return ErrMcpOAuthGrantTokenLinkCorrupt
		}
		var token Token
		if err := tx.Where("id = ? AND source = ? AND oauth_grant_id = ?", *grant.DedicatedTokenId, TokenSourceMcpOAuth, grant.PublicID).
			First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMcpOAuthGrantTokenLinkCorrupt
			}
			return err
		}
		if token.Status != common.TokenStatusEnabled {
			return ErrMcpOAuthGrantUnavailable
		}
		if now > 0 && grant.LastUsedAt < now {
			if err := tx.Model(&grant).Updates(map[string]any{
				"last_used_at": now,
				"updated_time": now,
			}).Error; err != nil {
				return err
			}
			grant.LastUsedAt = now
		}
		identity = McpOAuthDataToolIdentity{
			UserID:        grant.UserID,
			GrantPublicID: grant.PublicID,
			ClientID:      grant.ClientID,
			Resource:      grant.Resource,
			Scopes:        grant.Scope,
			Token:         token,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &identity, nil
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
