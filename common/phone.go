package common

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrInvalidPhoneNumber = errors.New("invalid phone number")

// NormalizePhoneNumber validates and normalizes an international phone number
// to the E.164 representation used as the stable identity for SMS verification.
func NormalizePhoneNumber(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "+") {
		return "", ErrInvalidPhoneNumber
	}

	var digits strings.Builder
	for _, r := range raw[1:] {
		switch {
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
			continue
		default:
			return "", ErrInvalidPhoneNumber
		}
	}

	value := digits.String()
	if len(value) < 8 || len(value) > 15 || value[0] == '0' {
		return "", ErrInvalidPhoneNumber
	}
	return fmt.Sprintf("+%s", value), nil
}
