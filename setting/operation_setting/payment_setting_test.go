package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPaymentAmountOptionsUseFiveDollarEntryTier(t *testing.T) {
	options := GetPaymentSetting().AmountOptions

	require.NotEmpty(t, options)
	require.Equal(t, 5, options[0])
	require.NotContains(t, options, 10)
}
