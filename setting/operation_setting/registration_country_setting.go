package operation_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// RegistrationCountrySetting controls country-based signup enforcement. Country
// codes are ISO 3166-1 alpha-2 codes (for example, MA = Morocco).
type RegistrationCountrySetting struct {
	Enabled              bool     `json:"enabled"`
	BlockedCountries     []string `json:"blocked_countries"`
	AutoDisableCountries []string `json:"auto_disable_countries"`
}

var registrationCountrySetting = RegistrationCountrySetting{
	Enabled:              true,
	BlockedCountries:     []string{"MA"},
	AutoDisableCountries: []string{},
}

func init() {
	config.GlobalConfig.Register("registration_country", &registrationCountrySetting)
}

func GetRegistrationCountrySetting() *RegistrationCountrySetting { return &registrationCountrySetting }

func normalizeCountries(countries []string) []string {
	seen := make(map[string]struct{}, len(countries))
	out := make([]string, 0, len(countries))
	for _, country := range countries {
		country = strings.ToUpper(strings.TrimSpace(country))
		if country == "" {
			continue
		}
		if _, ok := seen[country]; ok {
			continue
		}
		seen[country] = struct{}{}
		out = append(out, country)
	}
	return out
}

// NormalizeAndValidate keeps hot-path checks deterministic and rejects malformed
// country values before they are persisted to the options table.
func (s *RegistrationCountrySetting) NormalizeAndValidate() error {
	if s == nil {
		return fmt.Errorf("registration country setting is nil")
	}
	for _, countries := range [][]string{s.BlockedCountries, s.AutoDisableCountries} {
		for _, country := range countries {
			country = strings.ToUpper(strings.TrimSpace(country))
			if len(country) != 2 || country[0] < 'A' || country[0] > 'Z' || country[1] < 'A' || country[1] > 'Z' {
				return fmt.Errorf("invalid ISO country code: %q", country)
			}
		}
	}
	s.BlockedCountries = normalizeCountries(s.BlockedCountries)
	s.AutoDisableCountries = normalizeCountries(s.AutoDisableCountries)
	return nil
}

func UpdateRegistrationCountrySettingFromMap(values map[string]string) error {
	serialized, err := common.Marshal(registrationCountrySetting)
	if err != nil {
		return err
	}
	var next RegistrationCountrySetting
	if err := common.Unmarshal(serialized, &next); err != nil {
		return err
	}
	if err := config.UpdateConfigFromMap(&next, values); err != nil {
		return err
	}
	if err := next.NormalizeAndValidate(); err != nil {
		return err
	}
	registrationCountrySetting = next
	return nil
}

func countryIn(countries []string, country string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	for _, candidate := range countries {
		if strings.EqualFold(candidate, country) {
			return true
		}
	}
	return false
}

func IsCountryBlocked(country string) bool {
	return registrationCountrySetting.Enabled && countryIn(registrationCountrySetting.BlockedCountries, country)
}

func IsCountryAutoDisabled(country string) bool {
	return registrationCountrySetting.Enabled && countryIn(registrationCountrySetting.AutoDisableCountries, country)
}
