package common

import (
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resetVerificationMap() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}

func resetRegistrationEmailVerificationStore() {
	registrationEmailVerificationMutex.Lock()
	defer registrationEmailVerificationMutex.Unlock()
	registrationEmailVerificationMap = make(map[string]verificationValue)
}

func withRedisDisabled(t *testing.T) {
	t.Helper()
	prev := RedisEnabled
	RedisEnabled = false
	t.Cleanup(func() { RedisEnabled = prev })
}

func TestVerifyCodeWithKeyMemory(t *testing.T) {
	withRedisDisabled(t)
	resetVerificationMap()

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	if !VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected correct code to verify")
	}
	if VerifyCodeWithKey("a@b.com", "000000", EmailVerificationPurpose) {
		t.Fatal("expected wrong code to fail")
	}
	if VerifyCodeWithKey("a@b.com", "123456", PasswordResetPurpose) {
		t.Fatal("expected different purpose to fail")
	}

	DeleteKey("a@b.com", EmailVerificationPurpose)
	if VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected deleted code to fail")
	}
}

func TestVerifyCodeWithKeyMemoryExpiry(t *testing.T) {
	withRedisDisabled(t)
	resetVerificationMap()

	prev := VerificationValidMinutes
	VerificationValidMinutes = 0
	t.Cleanup(func() { VerificationValidMinutes = prev })

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	if VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected expired code to fail")
	}
}

// Regression test: with Redis enabled, a code registered by one instance must
// verify on another instance whose in-memory map is empty.
func TestVerifyCodeWithKeyRedisCrossInstance(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	// simulate the request landing on a different instance
	resetVerificationMap()

	if !VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected code stored in Redis to verify on another instance")
	}
	if VerifyCodeWithKey("a@b.com", "000000", EmailVerificationPurpose) {
		t.Fatal("expected wrong code to fail")
	}
	if VerifyCodeWithKey("a@b.com", "123456", PasswordResetPurpose) {
		t.Fatal("expected different purpose to fail")
	}

	DeleteKey("a@b.com", EmailVerificationPurpose)
	if VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected deleted code to fail")
	}
}

func TestVerifyCodeWithKeyRedisExpiry(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	resetVerificationMap()

	// must verify via Redis first, so the expiry below exercises the real TTL
	if !VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected code to verify before expiry")
	}

	mr.FastForward(time.Duration(VerificationValidMinutes)*time.Minute + time.Second)
	if VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected expired code to fail")
	}
}

// When Redis is enabled but unreachable, registration must fall back to the
// in-memory map so single-instance deployments keep working.
func TestVerifyCodeWithKeyRedisDownFallback(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()
	resetVerificationMap()

	mr.Close() // Redis becomes unreachable

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	if !VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected memory fallback to verify when Redis is down")
	}
}

// RedisEnabled defaults to true before InitRedisClient runs, leaving RDB nil;
// the verification functions must use the memory path instead of panicking.
func TestVerifyCodeWithKeyNilRDBFallsBackToMemory(t *testing.T) {
	prevEnabled, prevRDB := RedisEnabled, RDB
	RedisEnabled, RDB = true, nil
	t.Cleanup(func() { RedisEnabled, RDB = prevEnabled, prevRDB })
	resetVerificationMap()

	RegisterVerificationCodeWithKey("a@b.com", "123456", EmailVerificationPurpose)
	if !VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected memory path to work with nil RDB")
	}
	DeleteKey("a@b.com", EmailVerificationPurpose)
	if VerifyCodeWithKey("a@b.com", "123456", EmailVerificationPurpose) {
		t.Fatal("expected deleted code to fail")
	}
}

func TestRegistrationEmailLinkMemoryReplacesPreviousToken(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	first, err := RegisterRegistrationEmailLink("  user@example.com  ")
	if err != nil {
		t.Fatalf("register first link: %v", err)
	}
	second, err := RegisterRegistrationEmailLink("user@example.com")
	if err != nil {
		t.Fatalf("register second link: %v", err)
	}
	if first == second {
		t.Fatal("expected independent random link tokens")
	}

	if _, ok, err := ResolveRegistrationEmailLink(first); err != nil || ok {
		t.Fatalf("old link should be invalid after resend, ok=%t err=%v", ok, err)
	}
	email, ok, err := ResolveRegistrationEmailLink(second)
	if err != nil {
		t.Fatalf("resolve current link: %v", err)
	}
	if !ok || email != "user@example.com" {
		t.Fatalf("resolve current link = (%q, %t), want trimmed email", email, ok)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(second)
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("token entropy bytes = %d, want 32", len(decoded))
	}
}

func TestRegistrationEmailGrantMemoryMatchesExactEmail(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("  user@example.com  ")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}
	if !VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("expected matching trimmed email to verify")
	}
	if VerifyRegistrationEmailGrant(grant, "other@example.com") {
		t.Fatal("grant must not verify a different email")
	}

	DeleteRegistrationEmailGrant(grant)
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("deleted grant must not verify")
	}
}

func TestConsumeRegistrationEmailGrantMemoryIsAtomic(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("user@example.com")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}

	var successful int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ConsumeRegistrationEmailGrant(grant, "user@example.com") {
				atomic.AddInt32(&successful, 1)
			}
		}()
	}
	wg.Wait()

	if successful != 1 {
		t.Fatalf("successful grant consumptions = %d, want 1", successful)
	}
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("consumed grant must not remain valid")
	}
}

func TestRegistrationEmailGrantMemoryReservationRollsBack(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("user@example.com")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}
	if !ReserveRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("expected grant reservation to succeed")
	}
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("reserved grant must not remain available")
	}
	if ReserveRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("concurrent grant reservation must fail")
	}
	if !RollbackRegistrationEmailGrantReservation(grant, "user@example.com") {
		t.Fatal("expected grant reservation rollback to succeed")
	}
	if !VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("rolled back grant must be available for retry")
	}
}

func TestRegistrationEmailMemorySetBoundsCredentialCount(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	previousLimit := registrationEmailMemoryMaxSize
	registrationEmailMemoryMaxSize = 3
	t.Cleanup(func() { registrationEmailMemoryMaxSize = previousLimit })

	registrationEmailVerificationMutex.Lock()
	for index := range 4 {
		registrationEmailMemorySet(fmt.Sprintf("credential-%d", index), "user@example.com")
	}
	stored := len(registrationEmailVerificationMap)
	registrationEmailVerificationMutex.Unlock()

	if stored != registrationEmailMemoryMaxSize {
		t.Fatalf("stored credentials = %d, want %d", stored, registrationEmailMemoryMaxSize)
	}
}

func TestRegistrationEmailMemorySetReclaimsExpiredCredentials(t *testing.T) {
	withRedisDisabled(t)
	resetRegistrationEmailVerificationStore()

	registrationEmailVerificationMutex.Lock()
	registrationEmailVerificationMap["expired-unused-link"] = verificationValue{
		code: "user@example.com",
		time: time.Now().Add(-registrationEmailCredentialTTL() - time.Second),
	}
	registrationEmailMemorySet("fresh-grant", "user@example.com")
	_, expiredStillStored := registrationEmailVerificationMap["expired-unused-link"]
	_, freshStored := registrationEmailVerificationMap["fresh-grant"]
	registrationEmailVerificationMutex.Unlock()

	if expiredStillStored {
		t.Fatal("expected an expired unreferenced credential to be reclaimed")
	}
	if !freshStored {
		t.Fatal("expected the newly inserted credential to remain stored")
	}
}

func TestRegistrationEmailLinkRedisCrossInstanceAndExpiry(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()
	resetRegistrationEmailVerificationStore()

	token, err := RegisterRegistrationEmailLink("user@example.com")
	if err != nil {
		t.Fatalf("register link: %v", err)
	}
	resetRegistrationEmailVerificationStore()

	email, ok, err := ResolveRegistrationEmailLink(token)
	if err != nil {
		t.Fatalf("resolve Redis link: %v", err)
	}
	if !ok || email != "user@example.com" {
		t.Fatalf("resolve Redis link = (%q, %t), want stored email", email, ok)
	}

	mr.FastForward(time.Duration(VerificationValidMinutes)*time.Minute + time.Second)
	if _, ok, err := ResolveRegistrationEmailLink(token); err != nil || ok {
		t.Fatalf("expired Redis link should be invalid, ok=%t err=%v", ok, err)
	}
}

func TestRegistrationEmailGrantRedisCrossInstance(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("user@example.com")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}
	resetRegistrationEmailVerificationStore()

	if !VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("expected Redis grant to verify on another instance")
	}
	DeleteRegistrationEmailGrant(grant)
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("deleted Redis grant must not verify")
	}
}

func TestConsumeRegistrationEmailGrantRedisIsAtomic(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("user@example.com")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}

	var successful int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ConsumeRegistrationEmailGrant(grant, "user@example.com") {
				atomic.AddInt32(&successful, 1)
			}
		}()
	}
	wg.Wait()

	if successful != 1 {
		t.Fatalf("successful Redis grant consumptions = %d, want 1", successful)
	}
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("consumed Redis grant must not remain valid")
	}
}

func TestRegistrationEmailGrantRedisReservationRollsBackAndCommits(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()
	resetRegistrationEmailVerificationStore()

	grant, err := RegisterRegistrationEmailGrant("user@example.com")
	if err != nil {
		t.Fatalf("register grant: %v", err)
	}
	if !ReserveRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("expected Redis grant reservation to succeed")
	}
	if !RollbackRegistrationEmailGrantReservation(grant, "user@example.com") {
		t.Fatal("expected Redis grant reservation rollback to succeed")
	}
	if !VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("rolled back Redis grant must be available for retry")
	}
	if !ReserveRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("expected rolled back Redis grant to be reservable")
	}
	CommitRegistrationEmailGrantReservation(grant)
	if VerifyRegistrationEmailGrant(grant, "user@example.com") {
		t.Fatal("committed grant reservation must remain consumed")
	}
	if RollbackRegistrationEmailGrantReservation(grant, "user@example.com") {
		t.Fatal("committed grant reservation must not roll back")
	}
}

func TestRegistrationEmailLinkRedisFailureFailsClosed(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()
	resetRegistrationEmailVerificationStore()
	mr.Close()

	if _, err := RegisterRegistrationEmailLink("user@example.com"); err == nil {
		t.Fatal("expected Redis write failure to fail closed")
	}
	if _, err := RegisterRegistrationEmailGrant("user@example.com"); err == nil {
		t.Fatal("expected Redis grant write failure to fail closed")
	}
}
