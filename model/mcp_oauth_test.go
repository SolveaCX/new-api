package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupMcpOAuthTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/mcp-oauth.db?_pragma=busy_timeout(10000)&_txlock=immediate"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Token{},
		&McpOAuthClient{},
		&McpOAuthGrant{},
		&McpOAuthAuthorizationCode{},
		&McpOAuthRefreshToken{},
	))
	DB = db
	t.Cleanup(func() {
		sqlDB.Close()
		DB = previousDB
	})
}

func createMcpOAuthGrantFixture(t *testing.T, publicID string, userID int) McpOAuthGrant {
	t.Helper()
	grant := McpOAuthGrant{
		PublicID:    publicID,
		ClientID:    "mcp_client_" + publicID,
		UserID:      userID,
		AccountID:   userID,
		Resource:    "https://mcp.example",
		DisplayName: "Example MCP",
		Status:      McpOAuthGrantStatusActive,
		Scope:       "tools:read",
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	require.NoError(t, DB.Create(&grant).Error)
	return grant
}

func createMcpOAuthDedicatedTokenFixture(t *testing.T, userID int, grantID string, key string) Token {
	t.Helper()
	token := Token{
		UserId:         userID,
		Name:           "MCP hidden token",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    100,
		AccessedTime:   100,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
		Source:         TokenSourceMcpOAuth,
		OAuthGrantId:   &grantID,
	}
	require.NoError(t, DB.Create(&token).Error)
	return token
}

func mcpOAuthTokenTemplate(key string) Token {
	return Token{
		Name:           "MCP OAuth dedicated token",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    101,
		AccessedTime:   101,
		ExpiredTime:    -1,
		RemainQuota:    500,
		UnlimitedQuota: false,
		Group:          "default",
	}
}

func TestMcpOAuthProvisionDedicatedTokenLinksGrantAndTokenOneToOne(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_one_to_one", user.Id)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-one"), 200)

	require.NoError(t, err)
	require.True(t, created)
	require.NotZero(t, token.Id)
	require.Equal(t, TokenSourceMcpOAuth, token.Source)
	require.Equal(t, grant.PublicID, *token.OAuthGrantId)
	require.Empty(t, token.Key, "provisioning must not return the dedicated token secret")

	var storedGrant McpOAuthGrant
	require.NoError(t, DB.First(&storedGrant, "public_id = ?", grant.PublicID).Error)
	require.NotNil(t, storedGrant.DedicatedTokenId)
	require.Equal(t, token.Id, *storedGrant.DedicatedTokenId)

	duplicate := mcpOAuthTokenTemplate("mcp-secret-duplicate")
	duplicate.UserId = user.Id
	duplicate.Source = TokenSourceMcpOAuth
	duplicate.OAuthGrantId = stringPtr(grant.PublicID)
	require.Error(t, DB.Create(&duplicate).Error, "unique OAuthGrantId must prevent a second token for the grant")
}

func TestMcpOAuthProvisionDedicatedTokenRetryReturnsExistingToken(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-retry-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_retry", user.Id)

	first, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-first"), 200)
	require.NoError(t, err)
	require.True(t, created)

	second, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-second"), 201)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.Id, second.Id)

	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("oauth_grant_id = ?", grant.PublicID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestMcpOAuthTokenManagementScopeHidesDedicatedTokens(t *testing.T) {
	setupMcpOAuthTestDB(t)
	normal := Token{
		UserId:         7,
		Name:           "public-token",
		Key:            "public-secret",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, DB.Create(&normal).Error)
	mcp := createMcpOAuthDedicatedTokenFixture(t, 7, "grant_hidden_management", "mcp-hidden-secret")

	allTokens, err := GetAllUserTokens(7, 0, 10, "")
	require.NoError(t, err)
	require.Len(t, allTokens, 1)
	require.Equal(t, normal.Id, allTokens[0].Id)

	searchByName, total, err := SearchUserTokens(7, "MCP hidden token", "", "", 0, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, searchByName)

	searchBySecret, total, err := SearchUserTokens(7, "", "mcp-hidden-secret", "", 0, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, searchBySecret)

	_, err = GetTokenByIds(mcp.Id, 7)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetTokenByKey(mcp.Key, false)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	count, err := CountUserTokens(7)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	groupCount, err := CountUserTokensByGroup(7, "default")
	require.NoError(t, err)
	require.EqualValues(t, 1, groupCount)
	stats, err := GetUserTokenStats(7)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Total)

	keys, err := GetTokenKeysByIds([]int{normal.Id, mcp.Id}, 7)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, normal.Key, keys[0].Key)

	newQuota := 1
	updated, err := BatchUpdateTokens(BatchUpdateTokensParams{
		Ids:         []int{mcp.Id},
		UserId:      7,
		RemainQuota: &newQuota,
	})
	require.ErrorIs(t, err, ErrTokenBatchInvalid)
	require.Zero(t, updated)

	deleted, err := BatchDeleteTokens([]int{mcp.Id}, 7)
	require.NoError(t, err)
	require.Zero(t, deleted)
	var stillStored Token
	require.NoError(t, DB.First(&stillStored, mcp.Id).Error)
	require.Equal(t, TokenSourceMcpOAuth, stillStored.Source)
}

func TestMcpOAuthGrantSchemaSupportsStatusesAndUniqueRefreshFamily(t *testing.T) {
	setupMcpOAuthTestDB(t)
	familyID := "family_unique_grant"
	pending := McpOAuthGrant{
		PublicID:             "grant_schema_pending",
		ClientID:             "client_schema",
		UserID:               11,
		AccountID:            11,
		Resource:             "https://mcp.example/a",
		DisplayName:          "Schema Pending",
		Status:               McpOAuthGrantStatusPending,
		RefreshTokenFamilyId: nil,
		CreatedTime:          100,
		UpdatedTime:          100,
		LastUsedAt:           0,
	}
	secondPending := pending
	secondPending.PublicID = "grant_schema_pending_2"
	secondPending.Resource = "https://mcp.example/b"
	require.NoError(t, DB.Create(&pending).Error)
	require.NoError(t, DB.Create(&secondPending).Error, "nil refresh family must not collide")

	active := pending
	active.Id = 0
	active.PublicID = "grant_schema_active"
	active.Status = McpOAuthGrantStatusActive
	active.RefreshTokenFamilyId = &familyID
	require.NoError(t, DB.Create(&active).Error)

	duplicateFamily := pending
	duplicateFamily.Id = 0
	duplicateFamily.PublicID = "grant_schema_duplicate_family"
	duplicateFamily.RefreshTokenFamilyId = &familyID
	require.Error(t, DB.Create(&duplicateFamily).Error)

	require.Equal(t, McpOAuthGrantStatusFailed, "failed")
	require.Equal(t, McpOAuthGrantStatusRevoked, "revoked")
}

func TestMcpOAuthClientIDColumnsAllowCIMDURL(t *testing.T) {
	grantSchema, err := schema.Parse(&McpOAuthGrant{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	clientSchema, err := schema.Parse(&McpOAuthClient{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	require.Equal(t, "varchar(512)", grantSchema.LookUpField("ClientID").TagSettings["TYPE"])
	require.Equal(t, "varchar(512)", clientSchema.LookUpField("PublicID").TagSettings["TYPE"])
}

func TestMcpOAuthProvisionPendingGrantActivatesAtomically(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-pending-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := McpOAuthGrant{
		PublicID:    "grant_pending_activate",
		ClientID:    "client_pending",
		UserID:      user.Id,
		AccountID:   user.Id,
		Resource:    "https://mcp.example",
		DisplayName: "Pending App",
		Status:      McpOAuthGrantStatusPending,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	require.NoError(t, DB.Create(&grant).Error)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-pending"), 200)

	require.NoError(t, err)
	require.True(t, created)
	require.NotZero(t, token.Id)
	var storedGrant McpOAuthGrant
	require.NoError(t, DB.First(&storedGrant, "public_id = ?", grant.PublicID).Error)
	require.Equal(t, McpOAuthGrantStatusActive, storedGrant.Status)
	require.Equal(t, int64(200), storedGrant.UpdatedTime)
	require.NotNil(t, storedGrant.DedicatedTokenId)
}

func TestMcpOAuthFailedGrantFailsClosed(t *testing.T) {
	setupMcpOAuthTestDB(t)
	grant := McpOAuthGrant{
		PublicID:    "grant_failed",
		ClientID:    "client_failed",
		UserID:      12,
		AccountID:   12,
		Resource:    "https://mcp.example",
		DisplayName: "Failed App",
		Status:      McpOAuthGrantStatusFailed,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	require.NoError(t, DB.Create(&grant).Error)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-failed"), 200)

	require.ErrorIs(t, err, ErrMcpOAuthGrantUnavailable)
	require.False(t, created)
	require.Nil(t, token)
}

func TestMcpOAuthRevokedGrantNeverReactivatesOrReceivesReplacementToken(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-revoked-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_revoked", user.Id)
	require.NoError(t, DB.Model(&grant).Updates(map[string]any{
		"status":     McpOAuthGrantStatusRevoked,
		"revoked_at": int64(200),
	}).Error)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-revoked"), 201)

	require.ErrorIs(t, err, ErrMcpOAuthGrantRevoked)
	require.False(t, created)
	require.Nil(t, token)

	var stored McpOAuthGrant
	require.NoError(t, DB.First(&stored, "public_id = ?", grant.PublicID).Error)
	require.Equal(t, McpOAuthGrantStatusRevoked, stored.Status)
	require.Nil(t, stored.DedicatedTokenId)
}

func TestMcpOAuthDedicatedTokenPointerCorruptionFailsClosed(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-corrupt-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	normal := Token{
		UserId:         user.Id,
		Name:           "normal-token",
		Key:            "normal-secret",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
	}
	require.NoError(t, DB.Create(&normal).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_corrupt_pointer", user.Id)
	require.NoError(t, DB.Model(&grant).Update("dedicated_token_id", normal.Id).Error)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-corrupt"), 200)
	require.ErrorIs(t, err, ErrMcpOAuthGrantTokenLinkCorrupt)
	require.False(t, created)
	require.Nil(t, token)

	revoked, err := RevokeMcpOAuthGrant(grant.PublicID, 201)
	require.ErrorIs(t, err, ErrMcpOAuthGrantTokenLinkCorrupt)
	require.False(t, revoked)
	var storedNormal Token
	require.NoError(t, DB.First(&storedNormal, normal.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, storedNormal.Status)
}

func TestMcpOAuthAuthorizationCodeConsumesAtomicallyOnce(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-code-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_code", user.Id)
	codeSecret := "plain-code-secret"
	code := McpOAuthAuthorizationCode{
		PublicID:        "code_once",
		GrantPublicID:   grant.PublicID,
		CodeHash:        HashMcpOAuthCredential(codeSecret),
		RedirectURI:     "https://client.example/callback",
		Scope:           "tools:read",
		CreatedTime:     100,
		ExpiresAt:       300,
		CodeChallenge:   "challenge",
		ChallengeMethod: "S256",
	}
	require.NoError(t, DB.Create(&code).Error)

	first, consumed, err := ConsumeMcpOAuthAuthorizationCode(HashMcpOAuthCredential(codeSecret), 200)
	require.NoError(t, err)
	require.True(t, consumed)
	require.Equal(t, code.PublicID, first.PublicID)
	require.Equal(t, int64(200), first.ConsumedAt)

	second, consumed, err := ConsumeMcpOAuthAuthorizationCode(HashMcpOAuthCredential(codeSecret), 201)
	require.ErrorIs(t, err, ErrMcpOAuthCredentialConsumed)
	require.False(t, consumed)
	require.Nil(t, second)
}

func TestMcpOAuthConcurrentRefreshRotationHasOneWinnerAndReplayRevokesFamily(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-refresh-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_refresh", user.Id)
	currentSecret := "refresh-current"
	current := McpOAuthRefreshToken{
		PublicID:      "refresh_current",
		GrantPublicID: grant.PublicID,
		FamilyID:      "family_refresh",
		TokenHash:     HashMcpOAuthCredential(currentSecret),
		Status:        McpOAuthRefreshTokenStatusActive,
		CreatedTime:   100,
		ExpiresAt:     1000,
	}
	require.NoError(t, DB.Create(&current).Error)

	type rotationResult struct {
		token   *McpOAuthRefreshToken
		rotated bool
		err     error
	}
	results := make(chan rotationResult, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := McpOAuthRefreshToken{
				PublicID:    "refresh_next_" + string(rune('a'+i)),
				FamilyID:    current.FamilyID,
				TokenHash:   HashMcpOAuthCredential("refresh-next-" + string(rune('a'+i))),
				CreatedTime: 200 + int64(i),
				ExpiresAt:   1000,
			}
			token, rotated, err := RotateMcpOAuthRefreshToken(HashMcpOAuthCredential(currentSecret), next, 300+int64(i))
			results <- rotationResult{token: token, rotated: rotated, err: err}
		}(i)
	}
	wg.Wait()
	close(results)

	winners := 0
	replays := 0
	var winnerNextID int
	for result := range results {
		if result.rotated {
			winners++
			require.NoError(t, result.err)
			require.NotNil(t, result.token)
			winnerNextID = result.token.Id
			continue
		}
		replays++
		require.ErrorIs(t, result.err, ErrMcpOAuthRefreshReplay)
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, replays)

	var family []McpOAuthRefreshToken
	require.NoError(t, DB.Where("family_id = ?", current.FamilyID).Order("id asc").Find(&family).Error)
	require.Len(t, family, 2)
	for _, token := range family {
		require.Equal(t, McpOAuthRefreshTokenStatusRevoked, token.Status)
		require.NotZero(t, token.RevokedAt)
	}
	require.NotZero(t, winnerNextID, "winner must have created a next-generation token before replay revoked it")
	var storedGrant McpOAuthGrant
	require.NoError(t, DB.First(&storedGrant, "public_id = ?", grant.PublicID).Error)
	require.NotNil(t, storedGrant.RefreshTokenFamilyId)
	require.Equal(t, current.FamilyID, *storedGrant.RefreshTokenFamilyId)
}

func TestMcpOAuthRevokeGrantDisablesLinkedTokenInSameTransactionOnlyOnce(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-revoke-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_revoke", user.Id)
	token, _, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, mcpOAuthTokenTemplate("mcp-secret-linked"), 200)
	require.NoError(t, err)
	other := Token{UserId: user.Id, Key: "other-token", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 100}
	require.NoError(t, DB.Create(&other).Error)
	refresh := McpOAuthRefreshToken{
		PublicID:      "refresh_revoke",
		GrantPublicID: grant.PublicID,
		FamilyID:      "family_revoke_not_grant_id",
		TokenHash:     HashMcpOAuthCredential("refresh-revoke"),
		Status:        McpOAuthRefreshTokenStatusActive,
		CreatedTime:   100,
		ExpiresAt:     1000,
	}
	require.NoError(t, DB.Create(&refresh).Error)

	revoked, err := RevokeMcpOAuthGrant(grant.PublicID, 300)
	require.NoError(t, err)
	require.True(t, revoked)

	revoked, err = RevokeMcpOAuthGrant(grant.PublicID, 301)
	require.NoError(t, err)
	require.False(t, revoked)

	var linked Token
	require.NoError(t, DB.First(&linked, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, linked.Status)
	var unaffected Token
	require.NoError(t, DB.First(&unaffected, other.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, unaffected.Status)
	var revokedRefresh McpOAuthRefreshToken
	require.NoError(t, DB.First(&revokedRefresh, refresh.Id).Error)
	require.Equal(t, McpOAuthRefreshTokenStatusRevoked, revokedRefresh.Status)
	require.Equal(t, int64(300), revokedRefresh.RevokedAt)
}

func TestMcpOAuthSensitiveHashesAreHiddenFromJSON(t *testing.T) {
	code := McpOAuthAuthorizationCode{CodeHash: HashMcpOAuthCredential("code-secret")}
	refresh := McpOAuthRefreshToken{TokenHash: HashMcpOAuthCredential("refresh-secret")}

	codeJSON, err := common.Marshal(code)
	require.NoError(t, err)
	refreshJSON, err := common.Marshal(refresh)
	require.NoError(t, err)

	require.NotContains(t, string(codeJSON), "code_hash")
	require.NotContains(t, string(codeJSON), code.CodeHash)
	require.NotContains(t, string(refreshJSON), "token_hash")
	require.NotContains(t, string(refreshJSON), refresh.TokenHash)
}

func TestMcpOAuthTokenSourceCannotValidateAsPublicBearer(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-bearer-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_bearer", user.Id)
	secret := "mcp-secret-bearer"
	tokenTemplate := mcpOAuthTokenTemplate(secret)
	token, _, err := ProvisionMcpOAuthGrantDedicatedToken(grant.PublicID, tokenTemplate, 200)
	require.NoError(t, err)

	var stored Token
	require.NoError(t, DB.First(&stored, token.Id).Error)
	require.Equal(t, secret, stored.Key)

	validated, err := ValidateUserToken(secret)
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.Nil(t, validated)
}

func TestMcpOAuthTokenSourceRejectionPrecedesLifecycleErrors(t *testing.T) {
	setupMcpOAuthTestDB(t)
	grantID := "grant_lifecycle_reject"
	token := Token{
		UserId:         1,
		Key:            "mcp-secret-exhausted",
		Status:         common.TokenStatusExhausted,
		ExpiredTime:    -1,
		RemainQuota:    0,
		UnlimitedQuota: false,
		Source:         TokenSourceMcpOAuth,
		OAuthGrantId:   &grantID,
	}
	require.NoError(t, DB.Create(&token).Error)

	validated, err := ValidateUserToken(token.Key)

	require.ErrorIs(t, err, ErrTokenInvalid)
	require.Nil(t, validated)
}

func stringPtr(value string) *string {
	return &value
}

func TestMcpOAuthMigrationModelsIncludeAllTables(t *testing.T) {
	models := orderedMigrationModels()
	names := make(map[string]bool, len(models))
	for _, model := range models {
		names[model.name] = true
	}

	require.True(t, names["McpOAuthClient"])
	require.True(t, names["McpOAuthGrant"])
	require.True(t, names["McpOAuthAuthorizationCode"])
	require.True(t, names["McpOAuthRefreshToken"])
}

func TestMcpOAuthExpiredAuthorizationCodeFailsClosed(t *testing.T) {
	setupMcpOAuthTestDB(t)
	code := McpOAuthAuthorizationCode{
		PublicID:  "code_expired",
		CodeHash:  HashMcpOAuthCredential("expired-code-secret"),
		ExpiresAt: 100,
	}
	require.NoError(t, DB.Create(&code).Error)

	consumed, ok, err := ConsumeMcpOAuthAuthorizationCode(code.CodeHash, 101)

	require.ErrorIs(t, err, ErrMcpOAuthCredentialExpired)
	require.False(t, ok)
	require.Nil(t, consumed)
	var stored McpOAuthAuthorizationCode
	require.NoError(t, DB.First(&stored, "public_id = ?", code.PublicID).Error)
	require.Zero(t, stored.ConsumedAt)
}

func TestMcpOAuthRefreshReplayForAlreadyRotatedTokenRevokesEntireFamily(t *testing.T) {
	setupMcpOAuthTestDB(t)
	user := User{Username: "mcp-replay-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	grant := createMcpOAuthGrantFixture(t, "grant_replay", user.Id)
	currentHash := HashMcpOAuthCredential("current-replay")
	current := McpOAuthRefreshToken{
		PublicID:      "refresh_replay_current",
		GrantPublicID: grant.PublicID,
		FamilyID:      "family_replay",
		TokenHash:     currentHash,
		Status:        McpOAuthRefreshTokenStatusActive,
		CreatedTime:   100,
		ExpiresAt:     1000,
	}
	require.NoError(t, DB.Create(&current).Error)
	next := McpOAuthRefreshToken{
		PublicID:    "refresh_replay_next",
		FamilyID:    current.FamilyID,
		TokenHash:   HashMcpOAuthCredential("next-replay"),
		CreatedTime: 200,
		ExpiresAt:   1000,
	}
	_, rotated, err := RotateMcpOAuthRefreshToken(currentHash, next, 200)
	require.NoError(t, err)
	require.True(t, rotated)

	_, rotated, err = RotateMcpOAuthRefreshToken(currentHash, McpOAuthRefreshToken{
		PublicID:    "refresh_replay_loser",
		FamilyID:    current.FamilyID,
		TokenHash:   HashMcpOAuthCredential("loser-replay"),
		CreatedTime: 201,
		ExpiresAt:   1000,
	}, 201)

	require.ErrorIs(t, err, ErrMcpOAuthRefreshReplay)
	require.False(t, rotated)
	var activeCount int64
	require.NoError(t, DB.Model(&McpOAuthRefreshToken{}).Where("family_id = ? AND status = ?", current.FamilyID, McpOAuthRefreshTokenStatusActive).Count(&activeCount).Error)
	require.Zero(t, activeCount)
}

func TestMcpOAuthConsumeUnknownAuthorizationCodeFailsClosed(t *testing.T) {
	setupMcpOAuthTestDB(t)

	code, consumed, err := ConsumeMcpOAuthAuthorizationCode(HashMcpOAuthCredential("missing"), 100)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.False(t, consumed)
	require.Nil(t, code)
}

func TestMcpOAuthProvisionUnknownGrantFailsClosed(t *testing.T) {
	setupMcpOAuthTestDB(t)

	token, created, err := ProvisionMcpOAuthGrantDedicatedToken("missing-grant", mcpOAuthTokenTemplate("missing-key"), 100)

	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	require.False(t, created)
	require.Nil(t, token)
}
