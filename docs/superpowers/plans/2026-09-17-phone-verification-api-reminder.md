# Phone Verification API Reminder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Once every 24 hours, answer the first eligible text-generation API request of a legacy PLG account that has no verified phone with a synthesized, well-formed 200 "model reply" asking the user to bind a phone number, without forwarding or billing that request.

**Architecture:** A second pure rule next to the existing gate rule decides eligibility; `TokenAuth` stamps a context flag; `controller.Relay` consults the flag right after request parsing, claims a 24h Redis slot (`SET NX EX`), and writes a per-format synthesized body. Everything is behind a default-off operator option. Every failure path falls through to normal relaying.

**Tech Stack:** Go 1.25, Gin, go-redis v8, miniredis (tests), go-i18n YAML locales, testify.

**Spec:** `docs/superpowers/specs/2026-09-17-phone-verification-api-reminder-design.md`

**Baseline:** `origin/main` = `5711f1e75`. 33 pre-existing failing tests in `model`/`service`/`controller`/`middleware` are listed in `C:\Users\11247\.claude\tmp\phone-reminder-baseline-failures.txt` (BytePlus asset / Stripe / DingTalk / SQLite migration). Do not count them as regressions.

**Repo rules to honour:** JSON through `common.Marshal`/`common.Unmarshal`; new backend i18n keys go into all nine locale files with real translations; commit author is `think-back` (already configured in this worktree).

---

### Task 1: Eligibility rule in `model`

**Files:**
- Modify: `model/phone_verification_policy.go`
- Test: `model/phone_verification_policy_test.go`

- [ ] **Step 1: Write the failing tests** (append to `model/phone_verification_policy_test.go`)

```go
func TestPhoneVerificationReminderEligibleForAccountOnlyTargetsLegacyUnverifiedPlgAccounts(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = true
	start := PhoneVerificationRolloutStart().Unix()

	tests := []struct {
		name            string
		group           string
		phoneVerifiedAt int64
		createdAt       int64
		eligible        bool
	}{
		{name: "legacy plg account created before rollout", group: "plg", createdAt: start - 1, eligible: true},
		{name: "legacy plg account created months ago", group: "plg", createdAt: start - 90*86400, eligible: true},
		{name: "legacy plg account without created_at", group: "plg", createdAt: 0, eligible: true},
		{name: "new plg account is gated, not reminded", group: "plg", createdAt: start, eligible: false},
		{name: "new plg account created later is gated, not reminded", group: "plg", createdAt: start + 86400, eligible: false},
		{name: "legacy plg account with verified phone", group: "plg", phoneVerifiedAt: start - 10, createdAt: start - 100, eligible: false},
		{name: "legacy enterprise account", group: "enterprise", createdAt: start - 100, eligible: false},
		{name: "legacy account with empty group", group: "", createdAt: start - 100, eligible: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.eligible, PhoneVerificationReminderEligibleForAccount(tt.group, tt.phoneVerifiedAt, tt.createdAt))
		})
	}
}

func TestPhoneVerificationReminderEligibleForAccountIsOffWhenFeatureDisabled(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = false
	require.False(t, PhoneVerificationReminderEligibleForAccount("plg", 0, PhoneVerificationRolloutStart().Unix()-60))
}

func TestUserBasePhoneVerificationReminderEligibleHandlesNil(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = true
	start := PhoneVerificationRolloutStart().Unix()

	var nilUser *UserBase
	require.False(t, nilUser.PhoneVerificationReminderEligible())
	require.True(t, (&UserBase{Group: "plg", CreatedAt: start - 1}).PhoneVerificationReminderEligible())
	require.False(t, (&UserBase{Group: "plg", CreatedAt: start}).PhoneVerificationReminderEligible())
	require.False(t, (&UserBase{Group: "plg", CreatedAt: start - 1, PhoneVerifiedAt: start}).PhoneVerificationReminderEligible())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./model/ -run 'PhoneVerificationReminder' -count=1`
Expected: build failure `undefined: PhoneVerificationReminderEligibleForAccount`.

- [ ] **Step 3: Implement the rule** (append to `model/phone_verification_policy.go`)

```go
// PhoneVerificationReminderEligibleForAccount answers "should THIS account be
// reminded, through the API, to bind a phone?". It is the complement of the
// gate for accounts that can bind but are never forced to:
//
//   - feature flag SMS_VERIFICATION_ENABLED must be on, otherwise there is
//     nothing to bind;
//   - only PLG accounts are subject to it;
//   - an account with a verified phone is done;
//   - accounts the gate already blocks (created at or after the rollout
//     start) are never reminded: they get the 403 instead.
//
// Whether the reminder is actually served is decided by the operator toggle
// and the 24h claim, both outside this package.
func PhoneVerificationReminderEligibleForAccount(group string, phoneVerifiedAt int64, createdAt int64) bool {
	if !common.SMSVerificationEnabled {
		return false
	}
	if group != plgUserGroup {
		return false
	}
	if phoneVerifiedAt != 0 {
		return false
	}
	return !PhoneVerificationRequiredForAccount(group, phoneVerifiedAt, createdAt)
}

// PhoneVerificationReminderEligible is the UserBase convenience wrapper used by
// the token-auth middleware.
func (user *UserBase) PhoneVerificationReminderEligible() bool {
	if user == nil {
		return false
	}
	return PhoneVerificationReminderEligibleForAccount(user.Group, user.PhoneVerifiedAt, user.CreatedAt)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./model/ -run 'PhoneVerification' -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add model/phone_verification_policy.go model/phone_verification_policy_test.go
git commit -m "feat(model): add phone verification reminder eligibility rule"
```

---

### Task 2: Operator toggle in `operation_setting`

**Files:**
- Create: `setting/operation_setting/phone_verification_setting.go`
- Test: `setting/operation_setting/phone_verification_setting_test.go`

- [ ] **Step 1: Write the failing test**

```go
package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestPhoneVerificationAPIReminderEnabledRequiresFeatureFlagAndToggle(t *testing.T) {
	prevEnabled := common.SMSVerificationEnabled
	prevSetting := phoneVerificationSetting.APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = prevEnabled
		phoneVerificationSetting.APIReminderEnabled = prevSetting
	})

	// Default ships off even with the feature flag on.
	common.SMSVerificationEnabled = true
	phoneVerificationSetting.APIReminderEnabled = false
	require.False(t, PhoneVerificationAPIReminderEnabled())

	// Both on -> enabled.
	phoneVerificationSetting.APIReminderEnabled = true
	require.True(t, PhoneVerificationAPIReminderEnabled())

	// Feature flag off -> disabled regardless of the toggle.
	common.SMSVerificationEnabled = false
	require.False(t, PhoneVerificationAPIReminderEnabled())
}

func TestPhoneVerificationSettingDefaultsToReminderOff(t *testing.T) {
	require.False(t, GetPhoneVerificationSetting().APIReminderEnabled)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./setting/operation_setting/ -run 'PhoneVerification' -count=1`
Expected: build failure `undefined: phoneVerificationSetting`.

- [ ] **Step 3: Implement the module**

```go
package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// PhoneVerificationSetting holds operator-controlled phone-verification
// behaviour. The feature flag SMS_VERIFICATION_ENABLED (env) still decides
// whether phone verification exists at all; these options only tune what the
// deployment does on top of it.
type PhoneVerificationSetting struct {
	// APIReminderEnabled serves legacy PLG accounts without a verified phone a
	// synthesized "please bind a phone" reply once every 24 hours on their
	// first text-generation API call. Ships off; enable through
	// `phone_verification_setting.api_reminder_enabled`.
	APIReminderEnabled bool `json:"api_reminder_enabled"`
}

var phoneVerificationSetting = PhoneVerificationSetting{
	APIReminderEnabled: false,
}

func init() {
	config.GlobalConfig.Register("phone_verification_setting", &phoneVerificationSetting)
}

func GetPhoneVerificationSetting() *PhoneVerificationSetting {
	return &phoneVerificationSetting
}

// PhoneVerificationAPIReminderEnabled reports whether the API reminder should
// run: the SMS feature must exist and the operator must have opted in.
func PhoneVerificationAPIReminderEnabled() bool {
	return common.SMSVerificationEnabled && GetPhoneVerificationSetting().APIReminderEnabled
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./setting/operation_setting/ -run 'PhoneVerification' -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add setting/operation_setting/phone_verification_setting.go setting/operation_setting/phone_verification_setting_test.go
git commit -m "feat(setting): add phone_verification_setting.api_reminder_enabled toggle"
```

---

### Task 3: Context key and `TokenAuth` wiring

**Files:**
- Modify: `constant/context_key.go` (append inside the final `const (...)` block, after `ContextKeyRequestSamplingEligible`)
- Modify: `middleware/auth.go` (after the phone gate block at the `if userCache.PhoneVerificationRequired() { ... }` statement)
- Test: `middleware/phone_verification_gate_test.go` (append, package `middleware`)
- Test: `middleware/phone_verification_reminder_flag_test.go` (new, package `middleware_test`)

- [ ] **Step 1: Write the failing unit test** (append to `middleware/phone_verification_gate_test.go`)

```go
func TestPhoneVerificationReminderEligibleHonoursToggleAndRule(t *testing.T) {
	originalSMS := common.SMSVerificationEnabled
	originalToggle := operation_setting.GetPhoneVerificationSetting().APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = originalSMS
		operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = originalToggle
	})
	common.SMSVerificationEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()
	legacy := &model.UserBase{Group: "plg", CreatedAt: start - 1}

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = false
	require.False(t, phoneVerificationReminderEligible(legacy))

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = true
	require.True(t, phoneVerificationReminderEligible(legacy))
	require.False(t, phoneVerificationReminderEligible(&model.UserBase{Group: "plg", CreatedAt: start}))
	require.False(t, phoneVerificationReminderEligible(&model.UserBase{Group: "enterprise", CreatedAt: start - 1}))
	require.False(t, phoneVerificationReminderEligible(nil))
}
```

Add `"github.com/QuantumNous/new-api/setting/operation_setting"` to that test file's imports.

- [ ] **Step 2: Write the failing end-to-end test** (new file `middleware/phone_verification_reminder_flag_test.go`)

```go
package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupPhoneReminderFlagDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.RedisEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}))

	t.Cleanup(func() {
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

// TokenAuth must stamp the reminder flag for legacy PLG accounts only, and the
// existing gate must keep answering new PLG accounts with 403 before the flag
// is ever considered.
func TestTokenAuthStampsPhoneVerificationReminderFlag(t *testing.T) {
	setupPhoneReminderFlagDB(t)
	gin.SetMode(gin.TestMode)

	originalSMS := common.SMSVerificationEnabled
	originalToggle := operation_setting.GetPhoneVerificationSetting().APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = originalSMS
		operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = originalToggle
	})
	common.SMSVerificationEnabled = true
	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()

	users := []struct {
		id        int
		group     string
		createdAt int64
		verified  int64
		key       string
	}{
		{id: 26001, group: "plg", createdAt: start - 1, key: "legacyplgreminder"},
		{id: 26002, group: "plg", createdAt: start, key: "newplggated"},
		{id: 26003, group: "plg", createdAt: start - 1, verified: start - 1, key: "legacyplgverified"},
		{id: 26004, group: "enterprise", createdAt: start - 1, key: "legacyenterprise"},
	}
	for _, u := range users {
		require.NoError(t, model.DB.Create(&model.User{
			Id: u.id, Username: fmt.Sprintf("reminder-user-%d", u.id), Password: "password",
			Group: u.group, Status: common.UserStatusEnabled, CreatedAt: u.createdAt,
			PhoneVerifiedAt: u.verified, IsEnterprise: u.group == "enterprise",
		}).Error)
		require.NoError(t, model.DB.Create(&model.Token{
			Id: u.id, UserId: u.id, Key: u.key, Status: common.TokenStatusEnabled,
			RemainQuota: 1000, ExpiredTime: -1,
		}).Error)
	}

	engine := gin.New()
	engine.POST("/v1/chat/completions", middleware.TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reminder": common.GetContextKeyBool(c, constant.ContextKeyPhoneVerificationReminderEligible)})
	})

	call := func(key string) (int, string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[]}`))
		request.Header.Set("Authorization", "Bearer sk-"+key)
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		return recorder.Code, recorder.Body.String()
	}

	code, body := call("legacyplgreminder")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":true}`, body)

	code, body = call("newplggated")
	require.Equal(t, http.StatusForbidden, code, body)
	require.Contains(t, body, `"notify"`)

	code, body = call("legacyplgverified")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)

	code, body = call("legacyenterprise")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = false
	code, body = call("legacyplgreminder")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./middleware/ -run 'PhoneVerificationReminder' -count=1`
Expected: build failure `undefined: phoneVerificationReminderEligible` / `constant.ContextKeyPhoneVerificationReminderEligible`.

- [ ] **Step 4: Add the context key** (in `constant/context_key.go`, after `ContextKeyRequestSamplingEligible`, inside the same `const` block)

```go
	// ContextKeyPhoneVerificationReminderEligible is set by TokenAuth when the
	// caller is a legacy PLG account without a verified phone and the operator
	// enabled the API reminder. controller.Relay reads it to decide whether to
	// answer the request with the synthesized "bind your phone" reply.
	ContextKeyPhoneVerificationReminderEligible ContextKey = "phone_verification_reminder_eligible"
```

- [ ] **Step 5: Wire the middleware** (in `middleware/auth.go`)

Add the helper next to `userCanUseGroups`:

```go
// phoneVerificationReminderEligible combines the operator toggle with the
// per-account rule. It runs only after the phone gate passed, so a gated
// account can never be flagged for a reminder.
func phoneVerificationReminderEligible(userCache *model.UserBase) bool {
	return operation_setting.PhoneVerificationAPIReminderEnabled() && userCache.PhoneVerificationReminderEligible()
}
```

Insert right after the phone-gate `if` block (after its closing `}` and before the "PLG users are always served from the plg group" comment):

```go
		// Legacy PLG accounts are exempt from the gate; flag them so the relay
		// can serve the once-per-24h "bind your phone" reminder instead of the
		// model reply. Reading the flag, claiming the slot and rendering all
		// happen in controller.Relay.
		if phoneVerificationReminderEligible(userCache) {
			common.SetContextKey(c, constant.ContextKeyPhoneVerificationReminderEligible, true)
		}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./middleware/ -run 'PhoneVerification|TokenAuthStamps' -count=1`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add constant/context_key.go middleware/auth.go middleware/phone_verification_gate_test.go middleware/phone_verification_reminder_flag_test.go
git commit -m "feat(auth): flag legacy PLG accounts for the phone verification API reminder"
```

---

### Task 4: 24-hour claim in `service`

**Files:**
- Create: `service/phone_verification_reminder.go`
- Test: `service/phone_verification_reminder_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func usePhoneReminderMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	mini := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = oldEnabled
		common.RDB = oldRDB
	})
	return mini
}

func resetPhoneReminderMemoryClaims(t *testing.T) {
	t.Helper()
	oldNow := phoneVerificationReminderNow
	phoneVerificationReminderMemory.Lock()
	oldClaims := phoneVerificationReminderMemory.claims
	phoneVerificationReminderMemory.claims = map[int]time.Time{}
	phoneVerificationReminderMemory.Unlock()
	t.Cleanup(func() {
		phoneVerificationReminderNow = oldNow
		phoneVerificationReminderMemory.Lock()
		phoneVerificationReminderMemory.claims = oldClaims
		phoneVerificationReminderMemory.Unlock()
	})
}

func TestClaimPhoneVerificationReminderWinsOncePerWindowOnRedis(t *testing.T) {
	mini := usePhoneReminderMiniredis(t)

	first, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.True(t, first)

	second, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.False(t, second)

	other, err := ClaimPhoneVerificationReminder(43)
	require.NoError(t, err)
	require.True(t, other)

	require.Equal(t, PhoneVerificationReminderWindow, mini.TTL("phone_reminder:api:42"))

	mini.FastForward(PhoneVerificationReminderWindow + time.Second)
	again, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.True(t, again)
}

func TestClaimPhoneVerificationReminderReportsRedisErrors(t *testing.T) {
	mini := usePhoneReminderMiniredis(t)
	mini.Close()

	claimed, err := ClaimPhoneVerificationReminder(42)
	require.Error(t, err)
	require.False(t, claimed)
}

func TestClaimPhoneVerificationReminderRejectsInvalidUser(t *testing.T) {
	usePhoneReminderMiniredis(t)
	claimed, err := ClaimPhoneVerificationReminder(0)
	require.Error(t, err)
	require.False(t, claimed)
}

func TestClaimPhoneVerificationReminderMemoryFallbackWinsOncePerWindow(t *testing.T) {
	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldEnabled
		common.RDB = oldRDB
	})
	resetPhoneReminderMemoryClaims(t)
	base := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	phoneVerificationReminderNow = func() time.Time { return base }

	first, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.True(t, first)

	second, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.False(t, second)

	phoneVerificationReminderNow = func() time.Time { return base.Add(PhoneVerificationReminderWindow - time.Minute) }
	stillHeld, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.False(t, stillHeld)

	phoneVerificationReminderNow = func() time.Time { return base.Add(PhoneVerificationReminderWindow + time.Second) }
	expired, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.True(t, expired)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./service/ -run 'ClaimPhoneVerificationReminder' -count=1`
Expected: build failure `undefined: ClaimPhoneVerificationReminder`.

- [ ] **Step 3: Implement the claim**

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./service/ -run 'ClaimPhoneVerificationReminder' -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add service/phone_verification_reminder.go service/phone_verification_reminder_test.go
git commit -m "feat(service): 24h claim for the phone verification API reminder"
```

---

### Task 5: i18n key and nine locales

**Files:**
- Modify: `i18n/keys.go` (inside the "User notification messages" `const` block, after `MsgNotifyPhoneVerificationRequiredForAPI`)
- Modify: `i18n/locales/{en,zh-CN,zh-TW,pt,es,fr,ru,ja,vi}.yaml` (directly under the existing `notify.phone_verification_required_for_api` line in each file)
- Test: `i18n/phone_i18n_test.go` (append)

- [ ] **Step 1: Write the failing test** (append to `i18n/phone_i18n_test.go`; add `"strings"` to imports)

```go
func TestPhoneVerificationReminderForAPIIsLocalizedAcrossSupportedLanguages(t *testing.T) {
	require.NoError(t, Init())

	data := map[string]any{"SystemName": "Flatkey", "Link": "https://console.flatkey.ai/profile"}
	langs := []string{LangEn, LangZhCN, LangZhTW, LangPt, LangEs, LangFr, LangRu, LangJa, LangVi}
	seen := map[string]string{}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			out := Translate(lang, MsgNotifyPhoneVerificationReminderForAPI, data)
			require.NotEqual(t, MsgNotifyPhoneVerificationReminderForAPI, out)
			require.NotContains(t, out, "{{")
			require.NotContains(t, out, "<no value>")
			require.Contains(t, out, "Flatkey")
			require.Contains(t, out, "https://console.flatkey.ai/profile")
			for other, text := range seen {
				require.NotEqual(t, text, out, "%s and %s share the same text", lang, other)
			}
			seen[lang] = out
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./i18n/ -run 'PhoneVerificationReminderForAPI' -count=1`
Expected: build failure `undefined: MsgNotifyPhoneVerificationReminderForAPI`.

- [ ] **Step 3: Add the key** (`i18n/keys.go`, after `MsgNotifyPhoneVerificationRequiredForAPI`)

```go
	// MsgNotifyPhoneVerificationReminderForAPI is the body of the synthesized
	// "model reply" served once per 24h to legacy PLG accounts without a
	// verified phone. Template data: {{.SystemName}} {{.Link}}.
	MsgNotifyPhoneVerificationReminderForAPI = "notify.phone_verification_reminder_for_api"
```

- [ ] **Step 4: Add the nine translations**, each on the line right after `notify.phone_verification_required_for_api` in its file:

`en.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[{{.SystemName}} notice] Your account has not bound a phone number yet. Please sign in to the console and bind one: {{.Link}} . This request was not forwarded to the model and was not billed; simply send it again. This notice appears at most once every 24 hours."
```

`zh-CN.yaml`
```yaml
notify.phone_verification_reminder_for_api: "【{{.SystemName}} 提示】您的账号尚未绑定手机号，请登录控制台完成绑定：{{.Link}} 。本次请求未转发给模型、不计费，请直接重新发送。此提示每 24 小时最多出现一次。"
```

`zh-TW.yaml`
```yaml
notify.phone_verification_reminder_for_api: "【{{.SystemName}} 提示】您的帳號尚未綁定手機號碼，請登入控制台完成綁定：{{.Link}} 。本次請求未轉送給模型、不計費，請直接重新傳送。此提示每 24 小時最多出現一次。"
```

`pt.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[Aviso do {{.SystemName}}] Sua conta ainda não vinculou um número de telefone. Acesse o console e vincule um: {{.Link}} . Esta solicitação não foi encaminhada ao modelo nem cobrada; basta enviá-la novamente. Este aviso aparece no máximo uma vez a cada 24 horas."
```

`es.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[Aviso de {{.SystemName}}] Tu cuenta aún no tiene un número de teléfono vinculado. Inicia sesión en la consola y vincula uno: {{.Link}} . Esta solicitud no se envió al modelo ni se facturó; simplemente envíala de nuevo. Este aviso aparece como máximo una vez cada 24 horas."
```

`fr.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[Avis {{.SystemName}}] Votre compte n’a pas encore de numéro de téléphone associé. Connectez-vous à la console pour en associer un : {{.Link}} . Cette requête n’a pas été transmise au modèle et n’a pas été facturée ; renvoyez-la simplement. Cet avis apparaît au plus une fois toutes les 24 heures."
```

`ru.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[Уведомление {{.SystemName}}] К вашему аккаунту ещё не привязан номер телефона. Войдите в консоль и привяжите его: {{.Link}} . Этот запрос не был передан модели и не тарифицировался — просто отправьте его снова. Это уведомление показывается не чаще одного раза в 24 часа."
```

`ja.yaml`
```yaml
notify.phone_verification_reminder_for_api: "【{{.SystemName}} からのお知らせ】お客様のアカウントにはまだ電話番号が登録されていません。コンソールにログインして登録してください：{{.Link}} 。このリクエストはモデルに転送されておらず、課金もされていません。そのまま再送信してください。このお知らせは 24 時間に最大 1 回表示されます。"
```

`vi.yaml`
```yaml
notify.phone_verification_reminder_for_api: "[Thông báo từ {{.SystemName}}] Tài khoản của bạn chưa liên kết số điện thoại. Vui lòng đăng nhập bảng điều khiển và liên kết: {{.Link}} . Yêu cầu này chưa được chuyển tới mô hình và không bị tính phí; bạn chỉ cần gửi lại. Thông báo này xuất hiện tối đa một lần mỗi 24 giờ."
```

- [ ] **Step 5: Run the i18n tests**

Run: `go test ./i18n/ -count=1`
Expected: `ok` (the new test plus every existing locale test).

- [ ] **Step 6: Commit**

```bash
git add i18n/keys.go i18n/locales/*.yaml i18n/phone_i18n_test.go
git commit -m "feat(i18n): phone verification API reminder text in nine locales"
```

---

### Task 6: Request eligibility and structured-output skip (controller)

**Files:**
- Create: `controller/relay_phone_verification_reminder.go`
- Test: `controller/relay_phone_verification_reminder_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func chatReq(t *testing.T, body string) *dto.GeneralOpenAIRequest {
	t.Helper()
	req := &dto.GeneralOpenAIRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func claudeReq(t *testing.T, body string) *dto.ClaudeRequest {
	t.Helper()
	req := &dto.ClaudeRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func responsesReq(t *testing.T, body string) *dto.OpenAIResponsesRequest {
	t.Helper()
	req := &dto.OpenAIResponsesRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func geminiReq(t *testing.T, body string) *dto.GeminiChatRequest {
	t.Helper()
	req := &dto.GeminiChatRequest{}
	require.NoError(t, json.Unmarshal([]byte(body), req))
	return req
}

func TestPhoneVerificationReminderKindSelectsOnlyTextGeneration(t *testing.T) {
	tests := []struct {
		name    string
		format  types.RelayFormat
		mode    int
		path    string
		request dto.Request
		want    phoneVerificationReminderKind
		ok      bool
	}{
		{name: "chat completions", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[{"role":"user","content":"hi"}]}`), want: phoneReminderKindOpenAIChat, ok: true},
		{name: "legacy completions", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeCompletions, path: "/v1/completions",
			request: chatReq(t, `{"model":"gpt-x","prompt":"hi"}`), ok: false},
		{name: "moderations", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeModerations, path: "/v1/moderations",
			request: chatReq(t, `{"model":"omni","input":"hi"}`), ok: false},
		{name: "embeddings", format: types.RelayFormatEmbedding, mode: relayconstant.RelayModeEmbeddings, path: "/v1/embeddings",
			request: &dto.EmbeddingRequest{}, ok: false},
		{name: "claude messages", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"claude-x","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`), want: phoneReminderKindClaude, ok: true},
		{name: "responses", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi"}`), want: phoneReminderKindResponses, ok: true},
		{name: "responses compact", format: types.RelayFormatOpenAIResponsesCompaction, mode: relayconstant.RelayModeResponsesCompact, path: "/v1/responses/compact",
			request: &dto.OpenAIResponsesCompactionRequest{}, ok: false},
		{name: "gemini generateContent", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:generateContent",
			request: geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`), want: phoneReminderKindGemini, ok: true},
		{name: "gemini streamGenerateContent under /v1", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1/models/gemini-x:streamGenerateContent",
			request: geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`), want: phoneReminderKindGemini, ok: true},
		{name: "gemini countTokens", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:countTokens",
			request: geminiReq(t, `{"contents":[]}`), ok: false},
		{name: "gemini batch", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:generateContent",
			request: geminiReq(t, `{"requests":[{"contents":[]}]}`), ok: false},
		{name: "image", format: types.RelayFormatOpenAIImage, mode: relayconstant.RelayModeImagesGenerations, path: "/v1/images/generations",
			request: &dto.ImageRequest{}, ok: false},
		{name: "mismatched request type", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: &dto.ClaudeRequest{}, ok: false},
		{name: "nil request", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: nil, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.format, RelayMode: tt.mode}
			got, ok := phoneVerificationReminderKindFor(tt.format, info, tt.path, tt.request)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPhoneVerificationReminderSkipsMachineReadableRequests(t *testing.T) {
	tests := []struct {
		name    string
		format  types.RelayFormat
		mode    int
		path    string
		request dto.Request
		skip    bool
	}{
		{name: "chat json_object", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"json_object"}}`), skip: true},
		{name: "chat json_schema", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"json_schema","json_schema":{"name":"x"}}}`), skip: true},
		{name: "chat text format", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"text"}}`), skip: false},
		{name: "chat tool_choice required", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tool_choice":"required"}`), skip: true},
		{name: "chat tool_choice named", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tool_choice":{"type":"function","function":{"name":"f"}}}`), skip: true},
		{name: "chat tool_choice auto", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tools":[{"type":"function","function":{"name":"f"}}],"tool_choice":"auto"}`), skip: false},
		{name: "claude tool_choice any", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"any"}}`), skip: true},
		{name: "claude tool_choice tool", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"tool","name":"f"}}`), skip: true},
		{name: "claude tool_choice auto", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"auto"}}`), skip: false},
		{name: "claude output_format", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"output_format":{"type":"json_schema","schema":{}}}`), skip: true},
		{name: "responses text json_schema", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"json_schema","name":"x","schema":{}}}}`), skip: true},
		{name: "responses text json_object", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"json_object"}}}`), skip: true},
		{name: "responses text plain", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"text"},"verbosity":"low"}}`), skip: false},
		{name: "responses tool_choice required", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":"required"}`), skip: true},
		{name: "responses tool_choice object", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":{"type":"function","name":"f"}}`), skip: true},
		{name: "responses tool_choice auto", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":"auto"}`), skip: false},
		{name: "gemini json mime", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseMimeType":"application/json"}}`), skip: true},
		{name: "gemini responseSchema", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseSchema":{"type":"OBJECT"}}}`), skip: true},
		{name: "gemini responseJsonSchema", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseJsonSchema":{"type":"object"}}}`), skip: true},
		{name: "gemini function mode ANY", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"toolConfig":{"functionCallingConfig":{"mode":"ANY"}}}`), skip: true},
		{name: "gemini function mode AUTO", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"toolConfig":{"functionCallingConfig":{"mode":"AUTO"}}}`), skip: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.format, RelayMode: tt.mode}
			_, ok := phoneVerificationReminderKindFor(tt.format, info, tt.path, tt.request)
			require.Equal(t, !tt.skip, ok)
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./controller/ -run 'PhoneVerificationReminder' -count=1`
Expected: build failure `undefined: phoneVerificationReminderKindFor`.

- [ ] **Step 3: Implement kind detection** (new file `controller/relay_phone_verification_reminder.go`; the synthesis half is added in Task 7)

```go
package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

// phoneVerificationReminderKind names the response shape the gateway can
// synthesize for a request. Only text-generation shapes are supported; every
// other request is relayed untouched.
type phoneVerificationReminderKind int

const (
	phoneReminderKindOpenAIChat phoneVerificationReminderKind = iota + 1
	phoneReminderKindClaude
	phoneReminderKindResponses
	phoneReminderKindGemini
)

// phoneVerificationReminderKindFor decides whether the request can be answered
// with a synthesized reminder and, if so, in which shape. It returns false for
// unsupported entry points and for requests that ask for machine-readable
// output (JSON modes, forced tool calls), where a prose reply would break the
// caller's pipeline on the spot.
func phoneVerificationReminderKindFor(relayFormat types.RelayFormat, info *relaycommon.RelayInfo, path string, request dto.Request) (phoneVerificationReminderKind, bool) {
	if info == nil || request == nil {
		return 0, false
	}
	switch relayFormat {
	case types.RelayFormatOpenAI:
		req, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok || info.RelayMode != relayconstant.RelayModeChatCompletions {
			return 0, false
		}
		if openAIChatWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindOpenAIChat, true
	case types.RelayFormatClaude:
		req, ok := request.(*dto.ClaudeRequest)
		if !ok || claudeWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindClaude, true
	case types.RelayFormatOpenAIResponses:
		req, ok := request.(*dto.OpenAIResponsesRequest)
		if !ok || responsesWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindResponses, true
	case types.RelayFormatGemini:
		req, ok := request.(*dto.GeminiChatRequest)
		if !ok || !isGeminiGenerateContentPath(path) || len(req.Requests) > 0 {
			return 0, false
		}
		if geminiWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindGemini, true
	}
	return 0, false
}

// isGeminiGenerateContentPath matches /v1beta/models/{m}:generateContent and
// :streamGenerateContent (also under /v1/models/). Embeddings, countTokens
// and batch endpoints share the route but are not text generation.
func isGeminiGenerateContentPath(path string) bool {
	if !strings.HasPrefix(path, "/v1beta/models/") && !strings.HasPrefix(path, "/v1/models/") {
		return false
	}
	actionIndex := strings.LastIndex(path, ":")
	if actionIndex < 0 {
		return false
	}
	action := path[actionIndex+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}

func toolChoiceForcesCall(toolChoice any) bool {
	switch v := toolChoice.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "required")
	case map[string]any:
		return len(v) > 0
	}
	return false
}

func openAIChatWantsMachineReadableOutput(req *dto.GeneralOpenAIRequest) bool {
	if req.ResponseFormat != nil {
		switch strings.ToLower(strings.TrimSpace(req.ResponseFormat.Type)) {
		case "json_object", "json_schema":
			return true
		}
	}
	return toolChoiceForcesCall(req.ToolChoice)
}

func claudeWantsMachineReadableOutput(req *dto.ClaudeRequest) bool {
	if len(req.OutputFormat) > 0 && string(req.OutputFormat) != "null" {
		return true
	}
	choice, ok := req.ToolChoice.(map[string]any)
	if !ok {
		return false
	}
	switch strings.ToLower(common.Interface2String(choice["type"])) {
	case "any", "tool":
		return true
	}
	return false
}

func responsesWantsMachineReadableOutput(req *dto.OpenAIResponsesRequest) bool {
	if len(req.Text) > 0 {
		var text struct {
			Format struct {
				Type string `json:"type"`
			} `json:"format"`
		}
		if err := common.Unmarshal(req.Text, &text); err == nil {
			switch strings.ToLower(strings.TrimSpace(text.Format.Type)) {
			case "json_object", "json_schema":
				return true
			}
		}
	}
	if len(req.ToolChoice) > 0 {
		var choice any
		if err := common.Unmarshal(req.ToolChoice, &choice); err == nil && toolChoiceForcesCall(choice) {
			return true
		}
	}
	return false
}

func geminiWantsMachineReadableOutput(req *dto.GeminiChatRequest) bool {
	cfg := req.GenerationConfig
	if strings.EqualFold(strings.TrimSpace(cfg.ResponseMimeType), "application/json") {
		return true
	}
	if cfg.ResponseSchema != nil {
		return true
	}
	if len(cfg.ResponseJsonSchema) > 0 && string(cfg.ResponseJsonSchema) != "null" {
		return true
	}
	if req.ToolConfig != nil && req.ToolConfig.FunctionCallingConfig != nil &&
		strings.EqualFold(string(req.ToolConfig.FunctionCallingConfig.Mode), "ANY") {
		return true
	}
	return false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./controller/ -run 'PhoneVerificationReminder' -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add controller/relay_phone_verification_reminder.go controller/relay_phone_verification_reminder_test.go
git commit -m "feat(relay): decide which requests can carry the phone verification reminder"
```

---

### Task 7: Synthesized bodies for the four formats

**Files:**
- Modify: `controller/relay_phone_verification_reminder.go`
- Test: `controller/relay_phone_verification_reminder_test.go` (append)

- [ ] **Step 1: Write the failing tests** (append; add `"net/http"`, `"net/http/httptest"`, `"strings"`, `"github.com/gin-gonic/gin"` to the test imports)

```go
func newPhoneReminderRecorder(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	return c, recorder
}

func sseDataLines(t *testing.T, body string) []string {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, strings.TrimPrefix(line, "data: "))
		}
	}
	return lines
}

const reminderText = "please bind your phone"

func TestWritePhoneVerificationReminderOpenAIChat(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindOpenAIChat, false, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		var resp dto.OpenAITextResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "chat.completion", resp.Object)
		require.Equal(t, "gpt-x", resp.Model)
		require.True(t, strings.HasPrefix(resp.Id, "chatcmpl-"))
		require.Len(t, resp.Choices, 1)
		require.Equal(t, "assistant", resp.Choices[0].Message.Role)
		require.Equal(t, reminderText, resp.Choices[0].Message.StringContent())
		require.Equal(t, "stop", resp.Choices[0].FinishReason)
		require.Equal(t, 0, resp.Usage.TotalTokens)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindOpenAIChat, true, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		lines := sseDataLines(t, rec.Body.String())
		require.Len(t, lines, 3)
		var first, second dto.ChatCompletionsStreamResponse
		require.NoError(t, common.Unmarshal([]byte(lines[0]), &first))
		require.NoError(t, common.Unmarshal([]byte(lines[1]), &second))
		require.Equal(t, "chat.completion.chunk", first.Object)
		require.Equal(t, "assistant", first.Choices[0].Delta.Role)
		require.Equal(t, reminderText, first.Choices[0].Delta.GetContentString())
		require.Equal(t, first.Id, second.Id)
		require.NotNil(t, second.Choices[0].FinishReason)
		require.Equal(t, "stop", *second.Choices[0].FinishReason)
		require.Equal(t, "[DONE]", lines[2])
	})
}

func TestWritePhoneVerificationReminderClaude(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindClaude, false, "claude-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.ClaudeResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "message", resp.Type)
		require.Equal(t, "assistant", resp.Role)
		require.Equal(t, "claude-x", resp.Model)
		require.True(t, strings.HasPrefix(resp.Id, "msg_"))
		require.Len(t, resp.Content, 1)
		require.Equal(t, "text", resp.Content[0].Type)
		require.Equal(t, reminderText, resp.Content[0].GetText())
		require.Equal(t, "end_turn", resp.StopReason)
		require.NotNil(t, resp.Usage)
		require.Equal(t, 0, resp.Usage.OutputTokens)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindClaude, true, "claude-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		for _, event := range []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"} {
			require.Contains(t, body, "event: "+event+"\n", body)
		}
		require.Less(t, strings.Index(body, "event: message_start"), strings.Index(body, "event: content_block_delta"))
		require.Less(t, strings.Index(body, "event: content_block_delta"), strings.Index(body, "event: message_stop"))
		lines := sseDataLines(t, body)
		require.Len(t, lines, 6)
		var delta dto.ClaudeResponse
		require.NoError(t, common.Unmarshal([]byte(lines[2]), &delta))
		require.Equal(t, "content_block_delta", delta.Type)
		require.Equal(t, "text_delta", delta.Delta.Type)
		require.Equal(t, reminderText, delta.Delta.GetText())
		var messageDelta dto.ClaudeResponse
		require.NoError(t, common.Unmarshal([]byte(lines[4]), &messageDelta))
		require.Equal(t, "end_turn", *messageDelta.Delta.StopReason)
	})
}

func TestWritePhoneVerificationReminderResponses(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindResponses, false, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.OpenAIResponsesResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "response", resp.Object)
		require.Equal(t, `"completed"`, string(resp.Status))
		require.True(t, strings.HasPrefix(resp.ID, "resp_"))
		require.Len(t, resp.Output, 1)
		require.Equal(t, "message", resp.Output[0].Type)
		require.Equal(t, "assistant", resp.Output[0].Role)
		require.Len(t, resp.Output[0].Content, 1)
		require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
		require.Equal(t, reminderText, resp.Output[0].Content[0].Text)
		require.NotNil(t, resp.Usage)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindResponses, true, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		events := []string{"response.created", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed"}
		last := -1
		for _, event := range events {
			idx := strings.Index(body, "event: "+event+"\n")
			require.Greater(t, idx, last, "event %s missing or out of order", event)
			last = idx
		}
		lines := sseDataLines(t, body)
		require.Len(t, lines, len(events))
		var delta map[string]any
		require.NoError(t, common.Unmarshal([]byte(lines[3]), &delta))
		require.Equal(t, reminderText, delta["delta"])
		var completed struct {
			Response dto.OpenAIResponsesResponse `json:"response"`
		}
		require.NoError(t, common.Unmarshal([]byte(lines[7]), &completed))
		require.Equal(t, `"completed"`, string(completed.Response.Status))
		require.Equal(t, reminderText, completed.Response.Output[0].Content[0].Text)
		require.NotContains(t, body, "[DONE]")
	})
}

func TestWritePhoneVerificationReminderGemini(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindGemini, false, "gemini-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.GeminiChatResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Candidates, 1)
		require.Equal(t, "model", resp.Candidates[0].Content.Role)
		require.Equal(t, reminderText, resp.Candidates[0].Content.Parts[0].Text)
		require.Equal(t, "STOP", *resp.Candidates[0].FinishReason)
		require.Equal(t, 0, resp.UsageMetadata.TotalTokenCount)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindGemini, true, "gemini-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		lines := sseDataLines(t, rec.Body.String())
		require.Len(t, lines, 1)
		var resp dto.GeminiChatResponse
		require.NoError(t, common.Unmarshal([]byte(lines[0]), &resp))
		require.Equal(t, reminderText, resp.Candidates[0].Content.Parts[0].Text)
		require.NotContains(t, rec.Body.String(), "[DONE]")
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./controller/ -run 'WritePhoneVerificationReminder' -count=1`
Expected: build failure `undefined: writePhoneVerificationReminder`.

- [ ] **Step 3: Implement the writers** (append to `controller/relay_phone_verification_reminder.go`; extend its imports with `"encoding/json"`, `"fmt"`, `"net/http"`, `"github.com/QuantumNous/new-api/constant"`, `"github.com/QuantumNous/new-api/relay/helper"`, `"github.com/gin-gonic/gin"`)

```go
// phoneVerificationNoticeHeader lets integrations recognise a synthesized
// reminder deterministically instead of pattern-matching the text.
const (
	phoneVerificationNoticeHeader = "X-Flatkey-Notice"
	phoneVerificationNoticeValue  = "phone_verification_reminder"
)

// writePhoneVerificationReminder writes a complete, successful response in the
// requested shape whose only content is the reminder text. Usage is zero and
// the finish reason is a normal end of turn.
func writePhoneVerificationReminder(c *gin.Context, kind phoneVerificationReminderKind, stream bool, modelName string, text string) error {
	c.Header(phoneVerificationNoticeHeader, phoneVerificationNoticeValue)
	now := common.GetTimestamp()
	switch kind {
	case phoneReminderKindOpenAIChat:
		return writePhoneReminderOpenAIChat(c, stream, modelName, text, now)
	case phoneReminderKindClaude:
		return writePhoneReminderClaude(c, stream, modelName, text)
	case phoneReminderKindResponses:
		return writePhoneReminderResponses(c, stream, modelName, text, now)
	case phoneReminderKindGemini:
		return writePhoneReminderGemini(c, stream, text)
	}
	return fmt.Errorf("unsupported phone verification reminder kind %d", kind)
}

func writePhoneReminderOpenAIChat(c *gin.Context, stream bool, modelName string, text string, now int64) error {
	id := "chatcmpl-" + common.GetRandomString(29)
	if !stream {
		c.JSON(http.StatusOK, dto.OpenAITextResponse{
			Id:      id,
			Model:   modelName,
			Object:  "chat.completion",
			Created: now,
			Choices: []dto.OpenAITextResponseChoice{{
				Index:        0,
				Message:      dto.Message{Role: "assistant", Content: text},
				FinishReason: constant.FinishReasonStop,
			}},
			Usage: dto.Usage{},
		})
		return nil
	}
	helper.SetEventStreamHeaders(c)
	content := dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   modelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0}},
	}
	content.Choices[0].Delta.Role = "assistant"
	content.Choices[0].Delta.SetContentString(text)
	if err := helper.ObjectData(c, content); err != nil {
		return err
	}
	stop := helper.GenerateStopResponse(id, now, modelName, constant.FinishReasonStop)
	stop.Usage = &dto.Usage{}
	if err := helper.ObjectData(c, stop); err != nil {
		return err
	}
	helper.Done(c)
	return nil
}

func writePhoneReminderClaude(c *gin.Context, stream bool, modelName string, text string) error {
	id := "msg_" + common.GetRandomString(24)
	textBlock := dto.ClaudeMediaMessage{Type: "text"}
	textBlock.SetText(text)
	if !stream {
		c.JSON(http.StatusOK, dto.ClaudeResponse{
			Id:         id,
			Type:       "message",
			Role:       "assistant",
			Model:      modelName,
			Content:    []dto.ClaudeMediaMessage{textBlock},
			StopReason: "end_turn",
			Usage:      &dto.ClaudeUsage{},
		})
		return nil
	}
	helper.SetEventStreamHeaders(c)
	start := &dto.ClaudeMediaMessage{Id: id, Type: "message", Role: "assistant", Model: modelName, Usage: &dto.ClaudeUsage{}}
	start.SetContent(make([]any, 0))
	emptyText := dto.ClaudeMediaMessage{Type: "text"}
	emptyText.SetText("")
	delta := dto.ClaudeMediaMessage{Type: "text_delta"}
	delta.SetText(text)
	events := []dto.ClaudeResponse{
		{Type: "message_start", Message: start},
		{Type: "content_block_start", Index: common.GetPointer(0), ContentBlock: &emptyText},
		{Type: "content_block_delta", Index: common.GetPointer(0), Delta: &delta},
		{Type: "content_block_stop", Index: common.GetPointer(0)},
		{Type: "message_delta", Usage: &dto.ClaudeUsage{}, Delta: &dto.ClaudeMediaMessage{StopReason: common.GetPointer("end_turn")}},
		{Type: "message_stop"},
	}
	for _, event := range events {
		if err := helper.ClaudeData(c, event); err != nil {
			return err
		}
	}
	return nil
}

func phoneReminderResponsesBody(id string, itemID string, modelName string, text string, now int64, status string, content []dto.ResponsesOutputContent) dto.OpenAIResponsesResponse {
	output := []dto.ResponsesOutput{}
	if content != nil {
		output = append(output, dto.ResponsesOutput{Type: "message", ID: itemID, Status: "completed", Role: "assistant", Content: content})
	}
	return dto.OpenAIResponsesResponse{
		ID:                 id,
		Object:             "response",
		CreatedAt:          int(now),
		Status:             json.RawMessage(`"` + status + `"`),
		Instructions:       json.RawMessage(`null`),
		Model:              modelName,
		Output:             output,
		PreviousResponseID: json.RawMessage(`null`),
		ToolChoice:         json.RawMessage(`"auto"`),
		Tools:              []map[string]any{},
		Truncation:         json.RawMessage(`"disabled"`),
		Usage:              &dto.Usage{},
		User:               json.RawMessage(`null`),
		Metadata:           json.RawMessage(`{}`),
	}
}

func writePhoneReminderResponses(c *gin.Context, stream bool, modelName string, text string, now int64) error {
	id := "resp_" + common.GetRandomString(24)
	itemID := "msg_" + common.GetRandomString(24)
	content := []dto.ResponsesOutputContent{{Type: "output_text", Text: text, Annotations: []interface{}{}}}
	if !stream {
		c.JSON(http.StatusOK, phoneReminderResponsesBody(id, itemID, modelName, text, now, "completed", content))
		return nil
	}
	helper.SetEventStreamHeaders(c)
	inProgress := phoneReminderResponsesBody(id, itemID, modelName, text, now, "in_progress", nil)
	completed := phoneReminderResponsesBody(id, itemID, modelName, text, now, "completed", content)
	emptyItem := map[string]any{"type": "message", "id": itemID, "status": "in_progress", "role": "assistant", "content": []any{}}
	doneItem := map[string]any{"type": "message", "id": itemID, "status": "completed", "role": "assistant", "content": content}
	emptyPart := map[string]any{"type": "output_text", "text": "", "annotations": []any{}}
	donePart := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
	events := []map[string]any{
		{"type": "response.created", "response": inProgress},
		{"type": "response.output_item.added", "output_index": 0, "item": emptyItem},
		{"type": "response.content_part.added", "item_id": itemID, "output_index": 0, "content_index": 0, "part": emptyPart},
		{"type": "response.output_text.delta", "item_id": itemID, "output_index": 0, "content_index": 0, "delta": text},
		{"type": "response.output_text.done", "item_id": itemID, "output_index": 0, "content_index": 0, "text": text},
		{"type": "response.content_part.done", "item_id": itemID, "output_index": 0, "content_index": 0, "part": donePart},
		{"type": "response.output_item.done", "output_index": 0, "item": doneItem},
		{"type": "response.completed", "response": completed},
	}
	for index, event := range events {
		event["sequence_number"] = index
		data, err := common.Marshal(event)
		if err != nil {
			return err
		}
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event["type"].(string)}, string(data))
	}
	return nil
}

func writePhoneReminderGemini(c *gin.Context, stream bool, text string) error {
	body := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content:       dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: text}}},
			FinishReason:  common.GetPointer("STOP"),
			Index:         0,
			SafetyRatings: []dto.GeminiChatSafetyRating{},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{},
	}
	if !stream {
		c.JSON(http.StatusOK, body)
		return nil
	}
	helper.SetEventStreamHeaders(c)
	return helper.ObjectData(c, body)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./controller/ -run 'PhoneVerificationReminder' -count=1`
Expected: `ok`. If the Claude stream assertion on `lines[4]` fails because `Delta.StopReason` is nil, check that `dto.ClaudeMediaMessage.StopReason` is `*string` (it is) and that the event order matches the list above.

- [ ] **Step 5: Commit**

```bash
git add controller/relay_phone_verification_reminder.go controller/relay_phone_verification_reminder_test.go
git commit -m "feat(relay): synthesize the phone verification reminder in four response shapes"
```

---

### Task 8: `maybeServePhoneVerificationReminder` and the `Relay` hook

**Files:**
- Modify: `controller/relay_phone_verification_reminder.go`
- Modify: `controller/relay.go` (after the `IsModelOfficiallyUnsupported` block, before `needSensitiveCheck := ...`)
- Test: `controller/relay_phone_verification_reminder_test.go` (append)

- [ ] **Step 1: Write the failing tests** (append; add `"errors"`, `"github.com/QuantumNous/new-api/constant"`, `"github.com/QuantumNous/new-api/i18n"`, `"github.com/QuantumNous/new-api/service"` to the test imports)

```go
func stubPhoneReminderClaim(t *testing.T, claimed bool, err error) *int {
	t.Helper()
	calls := 0
	original := claimPhoneVerificationReminder
	claimPhoneVerificationReminder = func(userId int) (bool, error) {
		calls++
		return claimed, err
	}
	t.Cleanup(func() { claimPhoneVerificationReminder = original })
	return &calls
}

func newPhoneReminderRelayContext(t *testing.T, path string, eligible bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newPhoneReminderRecorder(t)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
	c.Set("id", 4242)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{Language: "zh-CN"})
	if eligible {
		common.SetContextKey(c, constant.ContextKeyPhoneVerificationReminderEligible, true)
	}
	return c, rec
}

func TestMaybeServePhoneVerificationReminderServesEligibleChatRequest(t *testing.T) {
	require.NoError(t, i18n.Init())
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/chat/completions", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "gpt-x", UserId: 4242}
	request := chatReq(t, `{"model":"gpt-x","messages":[{"role":"user","content":"hi"}]}`)

	served := maybeServePhoneVerificationReminder(c, types.RelayFormatOpenAI, info, request)

	require.True(t, served)
	require.Equal(t, 1, *calls)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, phoneVerificationNoticeValue, rec.Header().Get(phoneVerificationNoticeHeader))
	var resp dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	text := resp.Choices[0].Message.StringContent()
	require.Contains(t, text, "尚未绑定手机号")
	require.Contains(t, text, "http://localhost:3000/")
	require.Contains(t, text, common.SystemName)
}

func TestMaybeServePhoneVerificationReminderIsNoopWithoutFlag(t *testing.T) {
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/chat/completions", false)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "gpt-x", UserId: 4242}
	request := chatReq(t, `{"model":"gpt-x","messages":[]}`)

	require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatOpenAI, info, request))
	require.Equal(t, 0, *calls)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestMaybeServePhoneVerificationReminderDoesNotClaimUnsupportedRequests(t *testing.T) {
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/embeddings", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatEmbedding, RelayMode: relayconstant.RelayModeEmbeddings, OriginModelName: "emb", UserId: 4242}

	require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatEmbedding, info, &dto.EmbeddingRequest{}))
	require.Equal(t, 0, *calls, "unsupported requests must not consume the daily slot")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestMaybeServePhoneVerificationReminderFallsThroughWhenClaimLostOrFails(t *testing.T) {
	for name, tc := range map[string]struct {
		claimed bool
		err     error
	}{
		"lost":  {claimed: false, err: nil},
		"error": {claimed: false, err: errors.New("redis down")},
	} {
		t.Run(name, func(t *testing.T) {
			calls := stubPhoneReminderClaim(t, tc.claimed, tc.err)
			c, rec := newPhoneReminderRelayContext(t, "/v1/messages", true)
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "claude-x", UserId: 4242}
			request := claudeReq(t, `{"model":"claude-x","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)

			require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatClaude, info, request))
			require.Equal(t, 1, *calls)
			require.False(t, c.Writer.Written())
			require.Empty(t, rec.Body.String())
			require.Empty(t, rec.Header().Get(phoneVerificationNoticeHeader))
		})
	}
}

func TestMaybeServePhoneVerificationReminderHonoursStreamFlagAndGeminiPath(t *testing.T) {
	require.NoError(t, i18n.Init())
	stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1beta/models/gemini-x:streamGenerateContent", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatGemini, RelayMode: relayconstant.RelayModeGemini, IsStream: true, OriginModelName: "gemini-x", UserId: 4242}
	request := geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)

	require.True(t, maybeServePhoneVerificationReminder(c, types.RelayFormatGemini, info, request))
	require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	require.Len(t, sseDataLines(t, rec.Body.String()), 1)
}

func TestClaimPhoneVerificationReminderDefaultsToServiceImplementation(t *testing.T) {
	require.NotNil(t, claimPhoneVerificationReminder)
	// Keep the indirection pointed at the real implementation; tests above
	// stub it and restore it.
	_ = service.ClaimPhoneVerificationReminder
}

func TestRelayCallsPhoneVerificationReminderBeforeBilling(t *testing.T) {
	source, err := os.ReadFile("relay.go")
	require.NoError(t, err)
	text := string(source)
	hook := strings.Index(text, "maybeServePhoneVerificationReminder(c, relayFormat, relayInfo, request)")
	require.Greater(t, hook, 0, "Relay must call the reminder hook")
	require.Less(t, strings.Index(text, "IsModelOfficiallyUnsupported(relayInfo.OriginModelName)"), hook)
	require.Less(t, hook, strings.Index(text, "needSensitiveCheck := setting.ShouldCheckPromptSensitive()"))
	require.Less(t, hook, strings.Index(text, "service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)"))
}
```

Add `"os"` to the test imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./controller/ -run 'MaybeServePhoneVerificationReminder|RelayCallsPhoneVerificationReminder|ClaimPhoneVerificationReminderDefaults' -count=1`
Expected: build failure `undefined: maybeServePhoneVerificationReminder`.

- [ ] **Step 3: Implement the orchestration** (append to `controller/relay_phone_verification_reminder.go`; extend imports with `"github.com/QuantumNous/new-api/i18n"`, `"github.com/QuantumNous/new-api/logger"`, `"github.com/QuantumNous/new-api/service"`, `"github.com/QuantumNous/new-api/setting/system_setting"`)

```go
// claimPhoneVerificationReminder is indirected so tests can stub the 24h slot.
var claimPhoneVerificationReminder = service.ClaimPhoneVerificationReminder

// phoneVerificationReminderLink points the user at the console page that
// hosts the account bindings tab (and, for PLG accounts, the binding dialog).
func phoneVerificationReminderLink() string {
	origin := system_setting.ResolveConsoleOrigin()
	if origin == "" {
		origin = strings.TrimRight(system_setting.ServerAddress, "/")
	}
	return strings.TrimRight(origin, "/") + common.ThemeAwarePath("/console/personal")
}

func phoneVerificationReminderText(c *gin.Context) string {
	return i18n.Translate(i18n.GetLangFromContext(c), i18n.MsgNotifyPhoneVerificationReminderForAPI, map[string]any{
		"SystemName": common.SystemName,
		"Link":       phoneVerificationReminderLink(),
	})
}

// maybeServePhoneVerificationReminder answers the request with the synthesized
// reminder when TokenAuth flagged the account, the request shape is supported
// and this call wins the user's 24h slot. It reports whether it wrote the
// response. Every other outcome, including any error, returns false without
// touching the response so the request is relayed as usual.
func maybeServePhoneVerificationReminder(c *gin.Context, relayFormat types.RelayFormat, info *relaycommon.RelayInfo, request dto.Request) bool {
	if c == nil || info == nil || !common.GetContextKeyBool(c, constant.ContextKeyPhoneVerificationReminderEligible) {
		return false
	}
	kind, ok := phoneVerificationReminderKindFor(relayFormat, info, c.Request.URL.Path, request)
	if !ok {
		return false
	}
	claimed, err := claimPhoneVerificationReminder(info.UserId)
	if err != nil {
		logger.LogWarn(c, fmt.Sprintf("phone verification reminder skipped for user %d: %s", info.UserId, err.Error()))
		return false
	}
	if !claimed {
		return false
	}
	if err := writePhoneVerificationReminder(c, kind, info.IsStream, info.OriginModelName, phoneVerificationReminderText(c)); err != nil {
		logger.LogError(c, fmt.Sprintf("phone verification reminder write failed for user %d: %s", info.UserId, err.Error()))
		return c.Writer.Written()
	}
	logger.LogInfo(c, fmt.Sprintf("phone verification reminder served: user_id=%d model=%s format=%s stream=%t", info.UserId, info.OriginModelName, relayFormat, info.IsStream))
	return true
}
```

- [ ] **Step 4: Hook it into `Relay`** (`controller/relay.go`, immediately after the `IsModelOfficiallyUnsupported` block)

```go
	// Legacy PLG accounts without a verified phone get, once per 24h, a
	// synthesized "bind your phone" reply instead of the model. The flag is set
	// by TokenAuth; nothing has been counted or charged yet at this point. The
	// distributor's deferred release frees any channel-concurrency lease when
	// this handler returns.
	if maybeServePhoneVerificationReminder(c, relayFormat, relayInfo, request) {
		return
	}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./controller/ -run 'PhoneVerificationReminder|RelayCallsPhoneVerificationReminder' -count=1`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add controller/relay_phone_verification_reminder.go controller/relay_phone_verification_reminder_test.go controller/relay.go
git commit -m "feat(relay): serve the phone verification reminder once per 24h to legacy PLG accounts"
```

---

### Task 9: Full verification, spec touch-up, final commit

**Files:**
- Modify: `docs/superpowers/specs/2026-09-17-phone-verification-api-reminder-design.md` (only if any implemented detail drifted from the text)

- [ ] **Step 1: Build and vet**

Run: `go build ./... && go vet ./controller/ ./middleware/ ./service/ ./model/ ./setting/operation_setting/ ./i18n/ ./constant/`
Expected: no output besides the known `web/classic/dist` embed warning from `go build`.

- [ ] **Step 2: Run the feature tests together**

Run: `go test ./model/ ./setting/operation_setting/ ./middleware/ ./service/ ./i18n/ ./controller/ -run 'PhoneVerification|TokenAuthStamps|ClaimPhoneVerificationReminder|WritePhoneVerificationReminder|MaybeServePhoneVerificationReminder|RelayCallsPhoneVerificationReminder' -count=1`
Expected: every package `ok`.

- [ ] **Step 3: Regression diff against the baseline**

Run:
```bash
go test ./model/ ./middleware/ ./controller/ ./service/ ./setting/... ./i18n/ ./constant/ 2>&1 | grep -oE "\-\-\- FAIL: [A-Za-z0-9_]+" | sort -u > /c/Users/11247/.claude/tmp/phone-reminder-after-failures.txt
comm -13 /c/Users/11247/.claude/tmp/phone-reminder-baseline-failures.txt /c/Users/11247/.claude/tmp/phone-reminder-after-failures.txt
```
Expected: `comm` prints nothing (no failure that is not in the baseline list).

- [ ] **Step 4: Frontend i18n is untouched**

This change adds no console strings, so `bun run i18n:sync` is not required. Confirm with `git diff --stat origin/main..HEAD -- web/ website/` printing nothing.

- [ ] **Step 5: Review the diff**

Run: `git diff --stat origin/main..HEAD`
Expected: only the files named in this plan plus the spec and this plan. No deletions larger than a few lines.

- [ ] **Step 6: Commit any spec drift**

```bash
git add docs/superpowers/specs/2026-09-17-phone-verification-api-reminder-design.md
git commit -m "docs: align phone verification API reminder spec with implementation"
```
(Skip if nothing changed.)
