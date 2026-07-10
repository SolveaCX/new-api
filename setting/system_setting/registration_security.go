package system_setting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type RegistrationSecuritySettings struct {
	DomainRiskEnabled           bool     `json:"domain_risk_enabled"`
	DomainRiskWindowHours       int      `json:"domain_risk_window_hours"`
	DomainRiskThreshold         int      `json:"domain_risk_threshold"`
	TrustedEmailDomains         []string `json:"trusted_email_domains"`
	RejectSubdomainEmailDomains bool     `json:"reject_subdomain_email_domains"`
}

var registrationSecuritySettings = RegistrationSecuritySettings{
	DomainRiskWindowHours: 24,
	DomainRiskThreshold:   10,
}

func init() {
	config.GlobalConfig.Register("registration_security", &registrationSecuritySettings)
}

func GetRegistrationSecuritySettings() RegistrationSecuritySettings {
	cfg := registrationSecuritySettings
	cfg.TrustedEmailDomains = append([]string(nil), registrationSecuritySettings.TrustedEmailDomains...)
	return cfg
}

func (s *RegistrationSecuritySettings) NormalizeAndValidate() error {
	if s.DomainRiskWindowHours < 1 {
		return fmt.Errorf("registration risk window must be at least 1 hour")
	}
	if s.DomainRiskThreshold < 2 {
		return fmt.Errorf("registration risk threshold must be at least 2")
	}
	seen := make(map[string]struct{}, len(s.TrustedEmailDomains))
	normalized := make([]string, 0, len(s.TrustedEmailDomains))
	for _, raw := range s.TrustedEmailDomains {
		domain, err := common.NormalizeEmailDomain("user@" + strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid trusted email domain %q", raw)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	sort.Strings(normalized)
	s.TrustedEmailDomains = normalized
	return nil
}

func (s RegistrationSecuritySettings) IsTrustedDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, trusted := range s.TrustedEmailDomains {
		if domain == trusted {
			return true
		}
	}
	return false
}
