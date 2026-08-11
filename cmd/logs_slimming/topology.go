package main

import "fmt"

type topologyStatus string

const (
	topologyPreCutover  topologyStatus = "PRE_CUTOVER"
	topologyPostCutover topologyStatus = "POST_CUTOVER"
	topologyUnknown     topologyStatus = "UNKNOWN"
)

type objectTopology struct {
	source bool
	target bool
	old    bool
}

type triggerTopology struct {
	forward                bool
	reverse                bool
	updateGuard            bool
	deleteGuard            bool
	updateGuardTable       string
	deleteGuardTable       string
	futureUpdateGuard      bool
	futureDeleteGuard      bool
	futureUpdateGuardTable string
	futureDeleteGuardTable string
}

func classifyTopology(t objectTopology) topologyStatus {
	switch {
	case t.source && t.target && !t.old:
		return topologyPreCutover
	case t.source && !t.target && t.old:
		return topologyPostCutover
	default:
		return topologyUnknown
	}
}

type recoveryStep string

const (
	stepRecordRollbackBase   recoveryStep = "record-rollback-base"
	stepDropForward          recoveryStep = "drop-forward"
	stepCreateReverse        recoveryStep = "create-reverse"
	stepReconcileRollbackGap recoveryStep = "reconcile-rollback-gap"
)

func recoveryPlan(status topologyStatus, triggers triggerTopology) ([]recoveryStep, error) {
	if triggers.forward && triggers.reverse {
		return nil, fmt.Errorf("unsafe trigger cycle: forward and reverse triggers coexist")
	}
	switch status {
	case topologyPreCutover:
		if triggers.reverse {
			return nil, fmt.Errorf("reverse trigger cannot exist before cutover")
		}
		return nil, nil
	case topologyPostCutover:
		var plan []recoveryStep
		if triggers.forward {
			plan = append(plan, stepRecordRollbackBase, stepDropForward, stepCreateReverse, stepReconcileRollbackGap)
			return plan, nil
		}
		if !triggers.reverse {
			plan = append(plan, stepRecordRollbackBase, stepCreateReverse, stepReconcileRollbackGap)
			return plan, nil
		}
		return []recoveryStep{stepReconcileRollbackGap}, nil
	default:
		return nil, fmt.Errorf("cannot recover unknown object topology")
	}
}

type cleanupStep string

const (
	cleanupDropForward    cleanupStep = "drop-forward"
	cleanupDropGuards     cleanupStep = "drop-guards"
	cleanupDropTarget     cleanupStep = "drop-target"
	cleanupDropCheckpoint cleanupStep = "drop-checkpoint"
)

func cleanupPlan(status topologyStatus, triggers triggerTopology, ownershipConfirmed bool) ([]cleanupStep, error) {
	if status != topologyPreCutover {
		return nil, fmt.Errorf("cleanup only supports stable pre-cutover topology, got %s", status)
	}
	if !ownershipConfirmed {
		return nil, fmt.Errorf("cleanup ownership is not confirmed")
	}
	if triggers.reverse {
		return nil, fmt.Errorf("cleanup refuses a reverse trigger in pre-cutover topology")
	}
	return []cleanupStep{cleanupDropForward, cleanupDropGuards, cleanupDropTarget, cleanupDropCheckpoint}, nil
}
