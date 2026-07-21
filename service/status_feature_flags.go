package service

import "github.com/QuantumNous/new-api/common"

func IsStatusCenterEnabled() bool {
	return common.GetEnvOrDefaultBool("STATUS_CENTER_ENABLED", false)
}

func IsStatusCenterShadowMode() bool {
	return common.GetEnvOrDefaultBool("STATUS_CENTER_SHADOW_MODE", false)
}

func IsStatusCenterPublicEnabled() bool {
	return common.GetEnvOrDefaultBool("STATUS_CENTER_PUBLIC_ENABLED", false) && !IsStatusCenterShadowMode()
}

func IsStatusCenterNotificationsEnabled() bool {
	return common.GetEnvOrDefaultBool("STATUS_CENTER_NOTIFICATIONS_ENABLED", false) && !IsStatusCenterShadowMode()
}
