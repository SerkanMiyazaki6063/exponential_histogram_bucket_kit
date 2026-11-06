package exponential_histogram_bucket_kit

import (
	"math"
	"sort"
)

// ExponentialHistogramBucketKit maintains a fixed-size window of the most
// recent latency samples and computes exponentially weighted percentiles over
// them. It is designed for streaming systems where latency distributions drift
// over time and we want recent data to matter more than stale data, but we
// cannot afford unbounded memory.
//
// The window is a ring buffer of fixed capacity. Each sample is assigned a
// weight that decays exponentially with its age in the stream. The decay
// factor is controlled by HalfLife: after HalfLife samples have arrived, the
// weight of an old sample is halved. Percentile queries sort the samples by
// value and accumulate weights until the requested fraction of total weight is
// reached.
//
// Trade-off: exact percentiles over a fixed window would require storing all
// samples and re-sorting on every query. This implementation stores a fixed
// number of samples and sorts only on query, which is O(n log n) per query but
// uses O(n) memory regardless of stream length. The exponential weighting means
// the result is an approximation of the recent distribution rather than an
// exact quantile of a sliding window. For latency monitoring this is usually
// preferable because it smooths out noise while still reacting to shifts.
//
// The zero value is not usable; use NewExponentialHistogramBucketKit.
type ExponentialHistogramBucketKit struct {
	capacity int
	halfLife float64
	lambda   float64 // decay constant: ln(2)/HalfLife
	data     []float64
	weights  []float64 // parallel to data; weight[i] = exp(-lambda * age[i])
	ages     []float64 // age in samples since arrival, 0 for newest
	next     int       // index where the next sample will be written
	size     int       // number of valid samples currently in the buffer
	totalAge float64   // sum of ages of all valid samples
}

// NewExponentialHistogramBucketKit creates a kit with the given capacity and
// half-life. Capacity must be > 0; HalfLife must be > 0. If either is invalid,
// the function panics, matching the standard library's convention for
// programmer errors (e.g. make with negative length).
func NewExponentialHistogramBucketKit(capacity int, halfLife float64) *ExponentialHistogramBucketKit {
	if capacity <= 0 {
		panic("exponential_histogram_bucket_kit: capacity must be positive")
	}
	if halfLife <= 0 {
		panic("exponential_histogram_bucket_kit: halfLife must be positive")
	}
	return &ExponentialHistogramBucketKit{
		capacity: capacity,
		halfLife: halfLife,
		lambda:   math.Ln2 / halfLife,
		data:     make([]float64, capacity),
		weights:  make([]float64, capacity),
		ages:     make([]float64, capacity),
	}
}

// Add inserts a new sample into the window. If the window is full, the oldest
// sample is evicted. The new sample has age 0 and weight 1. All existing
// samples age by 1 and their weights are multiplied by exp(-lambda).
func (k *ExponentialHistogramBucketKit) Add(value float64) {
	// Age all existing samples and decay their weights.
	decay := math.Exp(-k.lambda)
	for i := 0; i < k.size; i++ {
		k.ages[i]++
		k.weights[i] *= decay
	}
	k.totalAge += float64(k.size)

	// If full, evict the oldest sample (which is at next because the ring
	// buffer overwrites the oldest).
	if k.size == k.capacity {
		oldest := k.next
		k.totalAge -= k.ages[oldest]
		// The slot will be overwritten below, so no need to clear.
	} else {
		k.size++
	}

	// Write the new sample at the next position.
	k.data[k.next] = value
	k.weights[k.next] = 1.0
	k.ages[k.next] = 0.0
	k.next = (k.next + 1) % k.capacity
}

// Percentile returns the value at the given percentile (0..1) using
// exponential weighting. For example, Percentile(0.5) returns the weighted
// median. The result is approximate because weights are used to select the
// sample. If the window is empty, Percentile returns NaN.
func (k *ExponentialHistogramBucketKit) Percentile(p float64) float64 {
	if k.size == 0 {
		return math.NaN()
	}
	if p <= 0 {
		return k.min()
	}
	if p >= 1 {
		return k.max()
	}

	// Build a slice of (value, weight) pairs for sorting. We allocate a new
	// slice each call because the data is stored in ring order and we need to
	// sort by value without disturbing the ring. This is the O(n log n) part.
	type sample struct {
		value  float64
		weight float64
	}
	samples := make([]sample, k.size)
	for i := 0; i < k.size; i++ {
		samples[i] = sample{value: k.data[i], weight: k.weights[i]}
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].value < samples[j].value
	})

	// Total weight is the sum of all weights. We compute it here rather than
	// maintaining it incrementally because weights decay and eviction changes
	// the set; recomputing is simpler and less error-prone.
	total := 0.0
	for _, s := range samples {
		total += s.weight
	}
	target := p * total
	cum := 0.0
	for _, s := range samples {
		cum += s.weight
		if cum >= target {
			return s.value
		}
	}
	return samples[len(samples)-1].value
}

// min returns the smallest value in the window. Used for p <= 0.
func (k *ExponentialHistogramBucketKit) min() float64 {
	m := k.data[0]
	for i := 1; i < k.size; i++ {
		if k.data[i] < m {
			m = k.data[i]
		}
	}
	return m
}

// max returns the largest value in the window. Used for p >= 1.
func (k *ExponentialHistogramBucketKit) max() float64 {
	m := k.data[0]
	for i := 1; i < k.size; i++ {
		if k.data[i] > m {
			m = k.data[i]
		}
	}
	return m
}

// Len returns the number of samples currently stored in the window.
func (k *ExponentialHistogramBucketKit) Len() int {
	return k.size
}

// Cap returns the maximum number of samples the window can hold.
func (k *ExponentialHistogramBucketKit) Cap() int {
	return k.capacity
}
