package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAndNormalizeCostCalculationOptionValue(t *testing.T) {
	valid := `{"assumptions":{"listPrice":10,"bonus":35,"coupon":5,"seedanceSellFactor":0.9,"otherSellFactor":0.8,"inputTokens":1000000,"outputTokens":1000000,"nonTokenUsage":1},"discounts":{"openai":0.6,"other":null},"fields":["Region"],"models":[{"name":"gpt-5.4-mini","vendor":"OpenAI","billingUnit":"USD/1M tokens","officialInput":0.75,"officialOutput":4.5,"officialNonToken":0,"costDiscount":null,"note":"","customFields":{"Region":"US"}}]}`

	normalized, err := validateAndNormalizeOptionValue("FlatkeyCostCalculation", valid)
	require.NoError(t, err)
	require.JSONEq(t, valid, normalized)
}

func TestValidateAndNormalizeCostCalculationOptionValueRejectsInvalidModels(t *testing.T) {
	base := `{"assumptions":{"listPrice":10,"bonus":35,"coupon":5,"seedanceSellFactor":0.9,"otherSellFactor":0.8,"inputTokens":1000000,"outputTokens":1000000,"nonTokenUsage":1},"discounts":{},"fields":[],"models":[%s]}`

	for name, model := range map[string]string{
		"invalid JSON":      `{"name":`,
		"empty name":        `{"name":"","vendor":"OpenAI","billingUnit":"USD","officialInput":0,"officialOutput":0,"officialNonToken":0}`,
		"negative price":    `{"name":"a","vendor":"OpenAI","billingUnit":"USD","officialInput":-1,"officialOutput":0,"officialNonToken":0}`,
		"negative discount": `{"name":"a","vendor":"OpenAI","billingUnit":"USD","officialInput":0,"officialOutput":0,"officialNonToken":0,"costDiscount":-0.1}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := validateAndNormalizeOptionValue("FlatkeyCostCalculation", fmt.Sprintf(base, model))
			require.Error(t, err)
		})
	}
}
