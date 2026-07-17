package service

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func statusSecretTestKey(seed byte) string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = seed + byte(index)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func statusSecretTestKeyring(t *testing.T) *StatusSecretKeyring {
	t.Helper()
	keyring, err := ParseStatusSecretKeyring(
		"old:"+statusSecretTestKey(1)+",active:"+statusSecretTestKey(33),
		"active",
	)
	require.NoError(t, err)
	return keyring
}

func TestStatusSecretKeyringUsesVersionedAESGCMAndDecryptsOldKeys(t *testing.T) {
	oldKeyring, err := ParseStatusSecretKeyring("old:"+statusSecretTestKey(1), "old")
	require.NoError(t, err)
	oldEnvelope, err := oldKeyring.Encrypt("https://hooks.example.com/status")
	require.NoError(t, err)
	require.Contains(t, oldEnvelope, "v1.old.")
	require.NotContains(t, oldEnvelope, "hooks.example.com")

	rotated := statusSecretTestKeyring(t)
	plaintext, err := rotated.Decrypt(oldEnvelope)
	require.NoError(t, err)
	require.Equal(t, "https://hooks.example.com/status", plaintext)

	activeEnvelope, err := rotated.Encrypt("signing-secret")
	require.NoError(t, err)
	require.Contains(t, activeEnvelope, "v1.active.")
	plaintext, err = rotated.Decrypt(activeEnvelope)
	require.NoError(t, err)
	require.Equal(t, "signing-secret", plaintext)
}

func TestStatusSecretKeyringRejectsInvalidConfigurationAndEnvelopes(t *testing.T) {
	invalidConfigurations := []struct {
		keys   string
		active string
	}{
		{keys: "broken", active: "broken"},
		{keys: "short:" + base64.StdEncoding.EncodeToString(make([]byte, 31)), active: "short"},
		{keys: "duplicate:" + statusSecretTestKey(1) + ",duplicate:" + statusSecretTestKey(2), active: "duplicate"},
		{keys: "known:" + statusSecretTestKey(1), active: "missing"},
	}
	for _, testCase := range invalidConfigurations {
		_, err := ParseStatusSecretKeyring(testCase.keys, testCase.active)
		require.Error(t, err)
	}

	keyring := statusSecretTestKeyring(t)
	envelope, err := keyring.Encrypt("secret")
	require.NoError(t, err)
	tampered := []byte(envelope)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	_, err = keyring.Decrypt(string(tampered))
	require.Error(t, err)
	_, err = keyring.Decrypt("v1.unknown.AAAA")
	require.Error(t, err)
	_, err = keyring.Decrypt("not-an-envelope")
	require.Error(t, err)
}

func TestStatusSecretMissingKeyringDisablesOnlyReversibleNotificationChannels(t *testing.T) {
	keyring, err := ParseStatusSecretKeyring("", "")
	require.NoError(t, err)
	require.False(t, keyring.Enabled())

	capabilities := StatusNotificationCapabilitiesFor(keyring)
	require.True(t, capabilities.Read)
	require.True(t, capabilities.Probe)
	require.True(t, capabilities.Email)
	require.False(t, capabilities.Webhook)
	require.False(t, capabilities.Discord)
	_, err = keyring.Encrypt("must not encrypt")
	require.ErrorIs(t, err, ErrStatusSecretKeyringDisabled)
}

func TestStatusSecretTokensAreRandomHashedAndConstantTimeVerified(t *testing.T) {
	first, err := GenerateStatusToken()
	require.NoError(t, err)
	second, err := GenerateStatusToken()
	require.NoError(t, err)
	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)

	hash := HashStatusToken(first)
	require.NotEqual(t, first, hash)
	require.True(t, VerifyStatusToken(hash, first))
	require.False(t, VerifyStatusToken(hash, second))
	require.False(t, VerifyStatusToken("malformed", first))
}

func TestStatusSecretFieldsNeverMarshalToJSON(t *testing.T) {
	subscriber := model.StatusSubscriber{
		IdentityHash:           "identity-hash",
		EncryptedEndpoint:      "encrypted-endpoint",
		EncryptedSigningSecret: "encrypted-signing-secret",
		VerificationTokenHash:  "verification-hash",
		ManageTokenHash:        "manage-hash",
		Status:                 model.StatusSubscriberPending,
	}
	payload, err := common.Marshal(subscriber)
	require.NoError(t, err)
	for _, secret := range []string{"identity-hash", "encrypted-endpoint", "encrypted-signing-secret", "verification-hash", "manage-hash"} {
		require.NotContains(t, string(payload), secret)
	}
}
