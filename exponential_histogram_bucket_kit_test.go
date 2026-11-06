package exponential_histogram_bucket_kit

import (
	"math"
	"testing"
)

func almostEqual(a, b, eps float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Abs(a-b) < eps
}

func TestNewPanicsOnInvalidCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for capacity <= 0")
		}
	}()
	NewExponentialHistogramBucketKit(0, 1.0)
}

func TestNewPanicsOnInvalidHalfLife(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for halfLife <= 0")
		}
	}()
	NewExponentialHistogramBucketKit(1, 0)
}

func TestEmptyPercentileReturnsNaN(t *testing.T) {
	k := NewExponentialHistogramBucketKit(4, 1.0)
	got := k.Percentile(0.5)
	if !math.IsNaN(got) {
		t.Fatalf("expected NaN for empty window, got %v", got)
	}
}

func TestAddAndLen(t *testing.T) {
	k := NewExponentialHistogramBucketKit(3, 1.0)
	if k.Len() != 0 {
		t.Fatalf("expected empty, got len %d", k.Len())
	}
	k.Add(1)
	k.Add(2)
	if k.Len() != 2 {
		t.Fatalf("expected len 2, got %d", k.Len())
	}
	k.Add(3)
	k.Add(4) // evicts 1
	if k.Len() != 3 {
		t.Fatalf("expected len 3 after eviction, got %d", k.Len())
	}
	if k.Cap() != 3 {
		t.Fatalf("expected cap 3, got %d", k.Cap())
	}
}

func TestPercentileBasic(t *testing.T) {
	// With halfLife = 1, weights decay fast. But for a single sample the
	// percentile is that sample.
	k := NewExponentialHistogramBucketKit(5, 1.0)
	k.Add(10)
	if got := k.Percentile(0.5); !almostEqual(got, 10, 1e-9) {
		t.Fatalf("expected 10, got %v", got)
	}
	if got := k.Percentile(0); !almostEqual(got, 10, 1e-9) {
		t.Fatalf("expected 10 for p=0, got %v", got)
	}
	if got := k.Percentile(1); !almostEqual(got, 10, 1e-9) {
		t.Fatalf("expected 10 for p=1, got %v", got)
	}
}

func TestPercentileWeightedMedian(t *testing.T) {
	// With halfLife large, weights are nearly equal, so median should be
	// close to the middle value of the three samples.
	k := NewExponentialHistogramBucketKit(3, 1000)
	k.Add(10)
	k.Add(20)
	k.Add(30)
	// Weights: 10 has age 2 -> exp(-2*ln2/1000) ~ 0.9986
	//          20 has age 1 -> ~0.9993
	//          30 has age 0 -> 1.0
	// Total ~ 2.9979, target 0.5 -> 1.49895. Cumulative: 10 -> 0.9986 (no),
	// +20 -> 1.9979 (yes) -> returns 20.
	got := k.Percentile(0.5)
	if !almostEqual(got, 20, 1e-9) {
		t.Fatalf("expected 20, got %v", got)
	}
}

func TestPercentileWithDecay(t *testing.T) {
	// halfLife = 1 means after one step weight halves. Add 10, then 20.
	// After adding 20, weight of 10 is 0.5, weight of 20 is 1.0.
	// Total = 1.5. p=0.5 -> target 0.75. Sorted: 10 (0.5), 20 (1.0).
	// Cumulative 0.5 < 0.75, then 1.5 >= 0.75 -> returns 20.
	k := NewExponentialHistogramBucketKit(2, 1.0)
	k.Add(10)
	k.Add(20)
	got := k.Percentile(0.5)
	if !almostEqual(got, 20, 1e-9) {
		t.Fatalf("expected 20, got %v", got)
	}
	// p=0.3 -> target 0.45. Cumulative 0.5 >= 0.45 -> returns 10.
	got = k.Percentile(0.3)
	if !almostEqual(got, 10, 1e-9) {
		t.Fatalf("expected 10, got %v", got)
	}
}

func TestEvictionAndWeightReset(t *testing.T) {
	// Capacity 2, halfLife 1. Add 10, 20, then 30 evicts 10.
	k := NewExponentialHistogramBucketKit(2, 1.0)
	k.Add(10)
	k.Add(20)
	k.Add(30) // evicts 10, remaining: 20 (age 1, weight 0.5), 30 (age 0, weight 1)
	// p=0.5 -> target 0.75 -> sorted 20(0.5), 30(1) -> cumulative 0.5 < 0.75 -> 30
	got := k.Percentile(0.5)
	if !almostEqual(got, 30, 1e-9) {
		t.Fatalf("expected 30, got %v", got)
	}
	// p=0.3 -> target 0.45 -> 0.5 >= 0.45 -> 20
	got = k.Percentile(0.3)
	if !almostEqual(got, 20, 1e-9) {
		t.Fatalf("expected 20, got %v", got)
	}
}

func TestPercentileOutOfRange(t *testing.T) {
	k := NewExponentialHistogramBucketKit(3, 1.0)
	k.Add(5)
	k.Add(15)
	k.Add(25)
	if got := k.Percentile(-0.5); !almostEqual(got, 5, 1e-9) {
		t.Fatalf("expected min 5 for p<0, got %v", got)
	}
	if got := k.Percentile(1.5); !almostEqual(got, 25, 1e-9) {
		t.Fatalf("expected max 25 for p>1, got %v", got)
	}
}
