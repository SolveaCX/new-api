package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// HiddenModelBlockedForIdentity reports whether the pricing hidden-model list
// must also block an API call. Only PLG identities are gated: the group is the
// user's identity group (not the token's routing group), and an empty group is
// PLG because every non-enterprise user is served from plg.
//
// Enterprise identities are never blocked here; hidden models stay callable
// for them exactly as before. Callers use this in one place for relay requests
// (middleware.Distribute) and in the /v1 model listing endpoints so PLG
// clients never discover a model they cannot call.
func HiddenModelBlockedForIdentity(identityGroup string, modelName string) bool {
	identityGroup = strings.TrimSpace(identityGroup)
	if identityGroup != "" && identityGroup != modelAccessPLGGroup {
		return false
	}
	return operation_setting.IsPricingHiddenModel(modelName)
}
