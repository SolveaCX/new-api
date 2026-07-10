package system_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistrationSecurityDefaults(t *testing.T) {
	cfg := GetRegistrationSecuritySettings()
	require.False(t, cfg.DomainRiskEnabled)
	require.Equal(t, 24, cfg.DomainRiskWindowHours)
	require.Equal(t, 10, cfg.DomainRiskThreshold)
	require.Empty(t, cfg.TrustedEmailDomains)
	require.False(t, cfg.RejectSubdomainEmailDomains)
}

func TestRegistrationSecurityNormalizeAndValidate(t *testing.T) {
	cfg := RegistrationSecuritySettings{
		DomainRiskWindowHours: 24,
		DomainRiskThreshold:   10,
		TrustedEmailDomains:   []string{" Example.com ", "example.com", "EXAMPLE.ORG"},
	}
	require.NoError(t, cfg.NormalizeAndValidate())
	require.Equal(t, []string{"example.com", "example.org"}, cfg.TrustedEmailDomains)
	require.True(t, cfg.IsTrustedDomain("EXAMPLE.COM"))
	require.False(t, cfg.IsTrustedDomain("mail.example.com"))

	cfg.DomainRiskThreshold = 1
	require.Error(t, cfg.NormalizeAndValidate())
	cfg.DomainRiskThreshold = 10
	cfg.DomainRiskWindowHours = 0
	require.Error(t, cfg.NormalizeAndValidate())
	cfg.DomainRiskWindowHours = 24
	cfg.TrustedEmailDomains = []string{"not a domain"}
	require.Error(t, cfg.NormalizeAndValidate())
}
