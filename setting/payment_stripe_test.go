package setting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentStripeDoesNotImportEncodingJson(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(".", "payment_stripe.go"))
	require.NoError(t, err)

	require.NotContains(t, string(source), `"encoding/json"`)
}

func TestStripeTopUpPriceIDForAmountUsesJsonWrapperParsedMap(t *testing.T) {
	originalTopUpPriceIds := StripeTopUpPriceIds
	t.Cleanup(func() {
		StripeTopUpPriceIds = originalTopUpPriceIds
	})

	StripeTopUpPriceIds = `{"5":" price_5 ","20":"price_20"}`

	require.Equal(t, "price_5", StripeTopUpPriceIDForAmount(5))
	require.Equal(t, "price_20", StripeTopUpPriceIDForAmount(20))
	require.Empty(t, StripeTopUpPriceIDForAmount(10))
	require.Empty(t, StripeTopUpPriceIDForAmount(200))
}

func TestStripeTopUpPriceIDForAmountUsesFiveDollarLegacyFallback(t *testing.T) {
	originalTopUpPriceIds := StripeTopUpPriceIds
	originalPriceId := StripePriceId
	originalPriceId20 := StripePriceId20
	originalPriceId200 := StripePriceId200
	t.Cleanup(func() {
		StripeTopUpPriceIds = originalTopUpPriceIds
		StripePriceId = originalPriceId
		StripePriceId20 = originalPriceId20
		StripePriceId200 = originalPriceId200
	})

	StripeTopUpPriceIds = ""
	StripePriceId = " price_5 "
	StripePriceId20 = " price_20 "
	StripePriceId200 = " price_200 "

	require.Equal(t, "price_5", StripeTopUpPriceIDForAmount(5))
	require.Equal(t, "price_20", StripeTopUpPriceIDForAmount(20))
	require.Equal(t, "price_200", StripeTopUpPriceIDForAmount(200))
	require.Empty(t, StripeTopUpPriceIDForAmount(10))
}
