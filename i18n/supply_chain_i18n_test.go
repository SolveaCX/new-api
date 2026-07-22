package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupplyChainMessagesExistInCompleteLocales(t *testing.T) {
	require.NoError(t, Init())
	keys := []string{
		MsgSupplyChainInvalidInput,
		MsgSupplyChainInvalidRate,
		MsgSupplyChainInvalidMoney,
		MsgSupplyChainIdempotencyKeyRequired,
		MsgSupplyChainNotFound,
		MsgSupplyChainConflict,
		MsgSupplyChainInternalError,
		MsgSupplyChainInvalidReportRange,
		MsgSupplyChainInvalidReportFilter,
		MsgSupplyChainReportUnavailable,
	}
	for _, language := range []string{"en", "zh-CN", "zh-TW", "pt"} {
		for _, key := range keys {
			translated := Translate(language, key)
			require.NotEmpty(t, translated, "%s %s", language, key)
			require.NotEqual(t, key, translated, "%s %s", language, key)
		}
	}
}
