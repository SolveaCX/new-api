package perfmetrics

import (
	"sync"
	"sync/atomic"
)

const availabilityBucketSeconds = int64(5 * 60)

var availabilityHotBuckets sync.Map

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
	eligible atomic.Int64
	success  atomic.Int64
}

func (bucket *atomicAvailabilityBucket) add(outcome AvailabilityOutcome) {
	if outcome != AvailabilityEligibleFailure && outcome != AvailabilityEligibleSuccess {
		return
	}
	bucket.eligible.Add(1)
	if outcome == AvailabilityEligibleSuccess {
		bucket.success.Add(1)
	}
}

func (bucket *atomicAvailabilityBucket) snapshot() availabilityCounters {
	return availabilityCounters{
		eligible: bucket.eligible.Load(),
		success:  bucket.success.Load(),
	}
}

func (bucket *atomicAvailabilityBucket) drain() availabilityCounters {
	return availabilityCounters{
		eligible: bucket.eligible.Swap(0),
		success:  bucket.success.Swap(0),
	}
}

func (bucket *atomicAvailabilityBucket) addCounters(counters availabilityCounters) {
	if counters.eligible != 0 {
		bucket.eligible.Add(counters.eligible)
	}
	if counters.success != 0 {
		bucket.success.Add(counters.success)
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
