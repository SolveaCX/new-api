package perfmetrics

import (
	"math"
	"sync"
	"sync/atomic"
)

const (
	availabilityBucketSeconds          = int64(5 * 60)
	availabilityRetentionSeconds       = int64(7 * 24 * 60 * 60)
	availabilityCleanupIntervalSeconds = int64(60 * 60)
)

var availabilityHotBuckets sync.Map
var statusAvailabilityEnabled atomic.Bool
var availabilityLastCleanupAt atomic.Int64

type availabilityBucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type availabilityCounters struct {
	eligible int64
	success  int64
}

type atomicAvailabilityBucket struct {
	counts atomic.Uint64
}

func (bucket *atomicAvailabilityBucket) add(outcome AvailabilityOutcome) {
	if outcome != AvailabilityEligibleFailure && outcome != AvailabilityEligibleSuccess {
		return
	}
	counters := availabilityCounters{eligible: 1}
	if outcome == AvailabilityEligibleSuccess {
		counters.success = 1
	}
	bucket.addCounters(counters)
}

func (bucket *atomicAvailabilityBucket) snapshot() availabilityCounters {
	return unpackAvailabilityCounters(bucket.counts.Load())
}

func (bucket *atomicAvailabilityBucket) drain() availabilityCounters {
	return unpackAvailabilityCounters(bucket.counts.Swap(0))
}

func (bucket *atomicAvailabilityBucket) addCounters(counters availabilityCounters) bool {
	if counters.eligible == 0 && counters.success == 0 {
		return true
	}
	if !validAvailabilityCounters(counters) {
		return false
	}
	for {
		currentWord := bucket.counts.Load()
		current := unpackAvailabilityCounters(currentWord)
		if counters.eligible > math.MaxUint32-current.eligible || counters.success > math.MaxUint32-current.success {
			return false
		}
		next := availabilityCounters{
			eligible: current.eligible + counters.eligible,
			success:  current.success + counters.success,
		}
		if !validAvailabilityCounters(next) {
			return false
		}
		if bucket.counts.CompareAndSwap(currentWord, packAvailabilityCounters(next)) {
			return true
		}
	}
}

func validAvailabilityCounters(counters availabilityCounters) bool {
	return counters.eligible >= 0 && counters.success >= 0 && counters.success <= counters.eligible &&
		counters.eligible <= math.MaxUint32 && counters.success <= math.MaxUint32
}

func packAvailabilityCounters(counters availabilityCounters) uint64 {
	return uint64(uint32(counters.eligible))<<32 | uint64(uint32(counters.success))
}

func unpackAvailabilityCounters(value uint64) availabilityCounters {
	return availabilityCounters{
		eligible: int64(value >> 32),
		success:  int64(value & math.MaxUint32),
	}
}

func recordAvailabilityAt(sample Sample, timestamp int64) {
	if sample.Availability != AvailabilityEligibleFailure && sample.Availability != AvailabilityEligibleSuccess {
		return
	}
	key := availabilityBucketKey{
		model:    sample.Model,
		group:    sample.Group,
		bucketTs: fixedAvailabilityBucketStart(timestamp),
	}
	actual, _ := availabilityHotBuckets.LoadOrStore(key, &atomicAvailabilityBucket{})
	actual.(*atomicAvailabilityBucket).add(sample.Availability)
}

func fixedAvailabilityBucketStart(timestamp int64) int64 {
	return timestamp - timestamp%availabilityBucketSeconds
}
