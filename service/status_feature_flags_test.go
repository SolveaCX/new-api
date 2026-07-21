package service

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestStatusFeatureFlagsRemainIndependentAndShadowSuppressesExternalEffects(t *testing.T) {
	t.Setenv("STATUS_CENTER_ENABLED", "false")
	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "true")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")

	require.False(t, IsStatusCenterEnabled())
	require.True(t, IsStatusCenterPublicEnabled(), "public reads can serve the last snapshot while evaluation is paused")
	require.True(t, IsStatusCenterNotificationsEnabled(), "the outbox can drain while evaluation is paused")
	require.False(t, IsStatusCenterShadowMode())

	t.Setenv("STATUS_CENTER_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "true")
	require.True(t, IsStatusCenterEnabled(), "shadow mode must still collect and evaluate evidence")
	require.True(t, IsStatusCenterShadowMode())
	require.False(t, IsStatusCenterPublicEnabled(), "shadow mode must not expose status publicly")
	require.False(t, IsStatusCenterNotificationsEnabled(), "shadow mode must not deliver notifications")
}

func TestStatusFeatureFlagsStartSchedulerAndDeliveryIndependently(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalSchedulerOnce := statusCenterTaskOnce
	originalDeliveryOnce := statusDeliveryTaskOnce
	originalSchedulerLaunch := statusCenterTaskLaunch
	originalDeliveryLaunch := statusDeliveryTaskLaunch
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		statusCenterTaskOnce = originalSchedulerOnce
		statusDeliveryTaskOnce = originalDeliveryOnce
		statusCenterTaskLaunch = originalSchedulerLaunch
		statusDeliveryTaskLaunch = originalDeliveryLaunch
	})

	common.IsMasterNode = true
	var schedulerLaunches atomic.Int64
	var deliveryLaunches atomic.Int64
	statusCenterTaskLaunch = func(*StatusScheduler) { schedulerLaunches.Add(1) }
	statusDeliveryTaskLaunch = func(StatusDeliveryWorker) { deliveryLaunches.Add(1) }

	statusCenterTaskOnce = &sync.Once{}
	statusDeliveryTaskOnce = &sync.Once{}
	t.Setenv("STATUS_CENTER_ENABLED", "false")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")
	t.Setenv("ROUTER_ORIGIN", "")
	require.True(t, StartStatusCenterTasks())
	require.Zero(t, schedulerLaunches.Load())
	require.EqualValues(t, 1, deliveryLaunches.Load())

	statusCenterTaskOnce = &sync.Once{}
	statusDeliveryTaskOnce = &sync.Once{}
	schedulerLaunches.Store(0)
	deliveryLaunches.Store(0)
	t.Setenv("STATUS_CENTER_ENABLED", "true")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "true")
	t.Setenv("ROUTER_ORIGIN", "https://router.flatkey.ai")
	require.True(t, StartStatusCenterTasks())
	require.EqualValues(t, 1, schedulerLaunches.Load())
	require.Zero(t, deliveryLaunches.Load())
}
