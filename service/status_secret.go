package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const statusSecretEnvelopeVersion = "v1"

var ErrStatusSecretKeyringDisabled = errors.New("status secret keyring is disabled")

type StatusSecretKeyring struct {
	keys        map[string][]byte
	activeKeyID string
}

type StatusNotificationCapabilities struct {
	Read    bool
	Probe   bool
	Email   bool
	Webhook bool
	Discord bool
}

func ParseStatusSecretKeyring(spec string, activeKeyID string) (*StatusSecretKeyring, error) {
	spec = strings.TrimSpace(spec)
	activeKeyID = strings.TrimSpace(activeKeyID)
	keyring := &StatusSecretKeyring{keys: make(map[string][]byte), activeKeyID: activeKeyID}
	if spec == "" && activeKeyID == "" {
		return keyring, nil
	}
	if spec == "" || activeKeyID == "" {
		return nil, errors.New("status secret keys and active key id must be configured together")
	}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		keyID, encodedKey, ok := strings.Cut(entry, ":")
		keyID = strings.TrimSpace(keyID)
		encodedKey = strings.TrimSpace(encodedKey)
		if !ok || !validStatusSecretKeyID(keyID) || encodedKey == "" {
			return nil, errors.New("invalid status secret key entry")
		}
		if _, exists := keyring.keys[keyID]; exists {
			return nil, fmt.Errorf("duplicate status secret key id %q", keyID)
		}
		key, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("status secret key %q must be a base64-encoded 32-byte key", keyID)
		}
		keyring.keys[keyID] = append([]byte(nil), key...)
	}
	if _, ok := keyring.keys[activeKeyID]; !ok {
		return nil, fmt.Errorf("status secret active key id %q is not configured", activeKeyID)
	}
	return keyring, nil
}

func LoadStatusSecretKeyringFromEnvironment() (*StatusSecretKeyring, error) {
	return ParseStatusSecretKeyring(os.Getenv("STATUS_SECRET_KEYS"), os.Getenv("STATUS_SECRET_ACTIVE_KEY_ID"))
}

func (keyring *StatusSecretKeyring) Enabled() bool {
	return keyring != nil && keyring.activeKeyID != "" && len(keyring.keys) > 0
}

func StatusNotificationCapabilitiesFor(keyring *StatusSecretKeyring) StatusNotificationCapabilities {
	enabled := keyring != nil && keyring.Enabled()
	return StatusNotificationCapabilities{
		Read: true, Probe: true, Email: true, Webhook: enabled, Discord: enabled,
	}
}

func (keyring *StatusSecretKeyring) Encrypt(plaintext string) (string, error) {
	if keyring == nil || !keyring.Enabled() {
		return "", ErrStatusSecretKeyringDisabled
	}
	block, err := aes.NewCipher(keyring.keys[keyring.activeKeyID])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate status secret nonce: %w", err)
	}
	authenticatedData := statusSecretAuthenticatedData(keyring.activeKeyID)
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), authenticatedData)
	payload := append(nonce, ciphertext...)
	return strings.Join([]string{
		statusSecretEnvelopeVersion,
		keyring.activeKeyID,
		base64.RawURLEncoding.EncodeToString(payload),
	}, "."), nil
}

func (keyring *StatusSecretKeyring) Decrypt(envelope string) (string, error) {
	if keyring == nil || len(keyring.keys) == 0 {
		return "", ErrStatusSecretKeyringDisabled
	}
	parts := strings.Split(envelope, ".")
	if len(parts) != 3 || parts[0] != statusSecretEnvelopeVersion || !validStatusSecretKeyID(parts[1]) || parts[2] == "" {
		return "", errors.New("malformed status secret envelope")
	}
	key, ok := keyring.keys[parts[1]]
	if !ok {
		return "", fmt.Errorf("unknown status secret key id %q", parts[1])
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != parts[2] {
		return "", errors.New("malformed status secret envelope payload")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < aead.NonceSize()+aead.Overhead() {
		return "", errors.New("malformed status secret envelope payload")
	}
	nonce := payload[:aead.NonceSize()]
	ciphertext := payload[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, statusSecretAuthenticatedData(parts[1]))
	if err != nil {
		return "", errors.New("status secret envelope authentication failed")
	}
	return string(plaintext), nil
}

func GenerateStatusToken() (string, error) {
	random := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate status token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func HashStatusToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func VerifyStatusToken(expectedHash string, token string) bool {
	expected, err := hex.DecodeString(expectedHash)
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(expected, actual[:]) == 1
}

func HashStatusIdentity(kind string, normalizedIdentity string) string {
	return common.GenerateHMAC(strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(normalizedIdentity))
}

func statusSecretAuthenticatedData(keyID string) []byte {
	return []byte(statusSecretEnvelopeVersion + "." + keyID)
}

func validStatusSecretKeyID(keyID string) bool {
	if keyID == "" || len(keyID) > 64 {
		return false
	}
	for _, character := range keyID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}
