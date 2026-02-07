package lblb

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func newTestBalancer(bucketCount int) *LBBalancer {
	balancer := New(nil, false)

	burst := 1000000
	average := 1000000
	period := 1
	priority := 1

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	for i := 0; i < bucketCount; i++ {
		name := fmt.Sprintf("srv-%d", i)
		balancer.Add(name, handler, &burst, &average, &period, &priority)
	}

	return balancer
}

func BenchmarkNextServer(b *testing.B) {
	bucketCounts := []int{1, 2, 4, 8, 16, 32, 64, 128}

	for _, bucketCount := range bucketCounts {
		b.Run(fmt.Sprintf("buckets_%d", bucketCount), func(b *testing.B) {
			balancer := newTestBalancer(bucketCount)
			b.ReportAllocs()

			var min time.Duration
			var max time.Duration
			var total time.Duration

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, err := balancer.nextServer()
				dur := time.Since(start)
				if err != nil {
					b.Fatal(err)
				}

				if i == 0 || dur < min {
					min = dur
				}
				if dur > max {
					max = dur
				}
				total += dur
			}

			avg := time.Duration(int64(total) / int64(b.N))
			b.ReportMetric(float64(min.Nanoseconds()), "ns_min")
			b.ReportMetric(float64(avg.Nanoseconds()), "ns_avg")
			b.ReportMetric(float64(max.Nanoseconds()), "ns_max")
		})
	}
}
