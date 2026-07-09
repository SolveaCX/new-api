package perfmetrics

import (
	"sync"
	"sync/atomic"
)

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model        string
	Group        string
	ChannelID    int
	LatencyMs    int64
	TtftMs       int64
	HasTtft      bool
	Success      bool
	OutputTokens int64
	GenerationMs int64
}

type QueryParams struct {
	Model string
	Group string
	Hours int
	// Groups, when non-nil, restricts results to these groups (Group must be
	// empty). MergeGroups collapses the matched groups into one "all" series
	// with counter-level (request-weighted) aggregation — averaging the
	// per-group series client-side would weight a 5-request group the same as
	// a 5M-request one.
	Groups      []string
	MergeGroups bool
}

type BucketPoint struct {
	Ts           int64   `json:"ts"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type GroupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	Series       []BucketPoint `json:"series"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Groups       []GroupResult `json:"groups"`
}

type ModelSummary struct {
	ModelName    string  `json:"model_name"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
	RequestCount int64   `json:"request_count"`
}

type SummaryAllResult struct {
	Models []ModelSummary `json:"models"`
}

type bucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type prometheusSeriesKey struct {
	model     string
	channelID int
	status    string
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	outputTokens   int64
	generationMs   int64
}

type atomicBucket struct {
	requestCount   atomic.Int64
	successCount   atomic.Int64
	totalLatencyMs atomic.Int64
	ttftSumMs      atomic.Int64
	ttftCount      atomic.Int64
	outputTokens   atomic.Int64
	generationMs   atomic.Int64
}

type prometheusCounters struct {
	buckets [prometheusLatencyBucketCount]int64
	count   int64
	sumMs   int64
}

type prometheusAtomicBucket struct {
	mu      sync.Mutex
	buckets [prometheusLatencyBucketCount]int64
	count   int64
	sumMs   int64
	closed  bool
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftSumMs.Add(sample.TtftMs)
		b.ttftCount.Add(1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.outputTokens.Add(sample.OutputTokens)
		b.generationMs.Add(sample.GenerationMs)
	}
}

func (b *atomicBucket) snapshot() counters {
	return counters{
		requestCount:   b.requestCount.Load(),
		successCount:   b.successCount.Load(),
		totalLatencyMs: b.totalLatencyMs.Load(),
		ttftSumMs:      b.ttftSumMs.Load(),
		ttftCount:      b.ttftCount.Load(),
		outputTokens:   b.outputTokens.Load(),
		generationMs:   b.generationMs.Load(),
	}
}

func (b *atomicBucket) drain() counters {
	return counters{
		requestCount:   b.requestCount.Swap(0),
		successCount:   b.successCount.Swap(0),
		totalLatencyMs: b.totalLatencyMs.Swap(0),
		ttftSumMs:      b.ttftSumMs.Swap(0),
		ttftCount:      b.ttftCount.Swap(0),
		outputTokens:   b.outputTokens.Swap(0),
		generationMs:   b.generationMs.Swap(0),
	}
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
	if c.ttftSumMs != 0 {
		b.ttftSumMs.Add(c.ttftSumMs)
	}
	if c.ttftCount != 0 {
		b.ttftCount.Add(c.ttftCount)
	}
	if c.outputTokens != 0 {
		b.outputTokens.Add(c.outputTokens)
	}
	if c.generationMs != 0 {
		b.generationMs.Add(c.generationMs)
	}
}

func (b *prometheusAtomicBucket) add(sample Sample) bool {
	latencyMs := sample.LatencyMs
	if latencyMs < 0 {
		latencyMs = 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	for i, upperBoundMs := range prometheusLatencyBucketUpperBoundsMs {
		if latencyMs <= upperBoundMs {
			b.buckets[i]++
		}
	}
	b.buckets[prometheusInfBucketIndex]++
	b.count++
	b.sumMs += latencyMs
	return true
}

func (b *prometheusAtomicBucket) snapshot() prometheusCounters {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := prometheusCounters{
		buckets: b.buckets,
		count:   b.count,
		sumMs:   b.sumMs,
	}
	return out
}

func (b *prometheusAtomicBucket) drain() prometheusCounters {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := prometheusCounters{
		buckets: b.buckets,
		count:   b.count,
		sumMs:   b.sumMs,
	}
	b.buckets = [prometheusLatencyBucketCount]int64{}
	b.count = 0
	b.sumMs = 0
	return out
}

func (b *prometheusAtomicBucket) addCounters(c prometheusCounters) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for i, value := range c.buckets {
		if value != 0 {
			b.buckets[i] += value
		}
	}
	b.count += c.count
	b.sumMs += c.sumMs
}

func (c prometheusCounters) isZero() bool {
	if c.count != 0 || c.sumMs != 0 {
		return false
	}
	for _, value := range c.buckets {
		if value != 0 {
			return false
		}
	}
	return true
}

func (c *prometheusCounters) add(other prometheusCounters) {
	for i, value := range other.buckets {
		c.buckets[i] += value
	}
	c.count += other.count
	c.sumMs += other.sumMs
}

func deleteEmptyPrometheusPendingBucket(rawKey any, bucket *prometheusAtomicBucket) {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	if bucket.count != 0 || bucket.sumMs != 0 {
		return
	}
	for _, value := range bucket.buckets {
		if value != 0 {
			return
		}
	}
	bucket.closed = true
	prometheusPendingBuckets.CompareAndDelete(rawKey, bucket)
}
