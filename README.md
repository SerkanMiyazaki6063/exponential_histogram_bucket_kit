# exponential_histogram_bucket_kit

A fixed-size bucket window for computing exponentially decaying percentiles over streaming latency data.

## Usage

```go
package main

import (
	"fmt"
	ehbk "github.com/your/package/exponential_histogram_bucket_kit"
)

func main() {
	kit := ehbk.NewExponentialHistogramBucketKit(100, 50.0)
	kit.Add(12.3)
	kit.Add(45.6)
	kit.Add(7.8)

	p50 := kit.Percentile(0.5)
	p99 := kit.Percentile(0.99)
	fmt.Printf("p50=%.2f p99=%.2f\n", p50, p99)
}
```

## Why this exists

Streaming latency monitoring needs percentiles that react to recent changes without storing an unbounded history. A naive sliding window of the last N samples gives equal weight to all samples in the window, so a single outlier can dominate for the entire window duration. Exponentially decaying weights let recent samples matter more, which matches how operators reason about latency: a spike five minutes ago is less interesting than one five seconds ago.

The trade-off is accuracy. Exact percentiles over a sliding window require storing every sample and re-sorting, which is O(N log N) per query and O(N) memory for the window size. This library stores a fixed number of samples (the capacity) and sorts only on query, so memory is O(capacity) regardless of stream length. The exponential weighting makes the result an approximation of the recent distribution rather than an exact quantile. For latency dashboards this is usually acceptable because the smoothing reduces noise while still tracking shifts.

## Edge cases

- The zero value of `ExponentialHistogramBucketKit` is not usable. Always construct with `NewExponentialHistogramBucketKit`.
- `Percentile` returns `NaN` when the window is empty.
- `Percentile` with `p <= 0` returns the minimum value; with `p >= 1` returns the maximum value.
- `NewExponentialHistogramBucketKit` panics if capacity or halfLife is not positive.
- Weights are recomputed on every `Add` by multiplying existing weights by `exp(-ln(2)/halfLife)`. This is O(capacity) per add. If capacity is large and add throughput is critical, consider a different data structure.

## API

- `func NewExponentialHistogramBucketKit(capacity int, halfLife float64) *ExponentialHistogramBucketKit`
- `func (k *ExponentialHistogramBucketKit) Add(value float64)`
- `func (k *ExponentialHistogramBucketKit) Percentile(p float64) float64`
- `func (k *ExponentialHistogramBucketKit) Len() int`
- `func (k *ExponentialHistogramBucketKit) Cap() int`
