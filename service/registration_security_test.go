package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestRegistrationSecurityPolicy(t *testing.T) {
	cfg := system_setting.RegistrationSecuritySettings{DomainRiskEnabled: true, DomainRiskWindowHours: 24, DomainRiskThreshold: 10, TrustedEmailDomains: []string{"trusted.com"}}
	decision, err := EvaluateRegistrationEmail("User@Example.COM", cfg, nil)
	require.NoError(t, err)
	require.Equal(t, "example.com", decision.Domain)
	require.True(t, decision.Policy.Enabled)

	trusted, err := EvaluateRegistrationEmail("user@trusted.com", cfg, nil)
	require.NoError(t, err)
	require.False(t, trusted.Policy.Enabled)

	child, err := EvaluateRegistrationEmail("user@mail.trusted.com", cfg, nil)
	require.NoError(t, err)
	require.True(t, child.Policy.Enabled)
}

func TestRegistrationSecurityRejectsSubdomainAndActiveBlock(t *testing.T) {
	cfg := system_setting.RegistrationSecuritySettings{DomainRiskWindowHours: 24, DomainRiskThreshold: 10, RejectSubdomainEmailDomains: true}
	_, err := EvaluateRegistrationEmail("user@mail.example.com", cfg, nil)
	require.ErrorIs(t, err, ErrSubdomainEmailRegistrationRejected)

	cfg.RejectSubdomainEmailDomains = false
	_, err = EvaluateRegistrationEmail("user@example.com", cfg, func(string) (bool, error) { return true, nil })
	require.ErrorIs(t, err, ErrRegistrationDomainUnavailable)
}

func TestRegistrationSecurityActiveBlockOverridesTrustedDomain(t *testing.T) {
	cfg := system_setting.RegistrationSecuritySettings{
		DomainRiskEnabled:     true,
		DomainRiskWindowHours: 24,
		DomainRiskThreshold:   10,
		TrustedEmailDomains:   []string{"trusted.example"},
	}

	_, err := EvaluateRegistrationEmail("user@trusted.example", cfg, func(string) (bool, error) {
		return true, nil
	})

	require.ErrorIs(t, err, ErrRegistrationDomainUnavailable)
}

func TestRegistrationSecurityAllowsEmailLessRegistrationWithoutCounting(t *testing.T) {
	cfg := system_setting.RegistrationSecuritySettings{DomainRiskEnabled: true, DomainRiskWindowHours: 24, DomainRiskThreshold: 10}

	decision, err := EvaluateRegistrationEmail("", cfg, func(string) (bool, error) {
		t.Fatal("email-less registration must not query domain blocks")
		return false, nil
	})

	require.NoError(t, err)
	require.Empty(t, decision.Domain)
	require.False(t, decision.Policy.Enabled)
}
