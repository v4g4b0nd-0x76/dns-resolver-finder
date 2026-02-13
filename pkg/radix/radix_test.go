package radix

import (
	"dns-resolver-finder/pkg/types"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIPTree_Race(t *testing.T) {
	tree := NewIPTree(100)
	var wg sync.WaitGroup

	workers := 10
	iterations := 100

	for i := range workers {
		wg.Add(4)

		go func(workerID int) {
			defer wg.Done()
			for j := range iterations {
				ip := fmt.Sprintf("192.168.%d.%d", workerID, j)
				tree.Insert(&types.Resolver{
					IP:      ip,
					Latency: time.Duration(j) * time.Millisecond,
				})
			}
		}(i)

		go func(workerID int) {
			defer wg.Done()
			for j := range iterations {
				ip := fmt.Sprintf("10.0.%d.%d", workerID, j)
				tree.ReplaceWorst(&types.Resolver{
					IP:      ip,
					Latency: time.Duration(j) * time.Microsecond,
				})
			}
		}(i)

		go func(workerID int) {
			defer wg.Done()
			for range iterations {
				_ = tree.GetAllSortedByLatency()
				_ = tree.SearchRange("192.168.0.0/16")
				_ = tree.Len()
			}
		}(i)

		go func(workerID int) {
			defer wg.Done()
			for j := range iterations {
				ip := fmt.Sprintf("192.168.%d.%d", workerID, j)
				tree.Delete(ip)
				_ = tree.Get(ip)
			}
		}(i)
	}

	wg.Wait()
}

func BenchmarkIPTree_Insert(b *testing.B) {
	tree := NewIPTree(b.N)
	resolvers := make([]*types.Resolver, b.N)
	for i := 0; i < b.N; i++ {
		resolvers[i] = &types.Resolver{
			IP:      fmt.Sprintf("%d.%d.%d.%d", byte(i>>24), byte(i>>16), byte(i>>8), byte(i)),
			Latency: time.Duration(i) * time.Millisecond,
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(resolvers[i])
	}
}

func BenchmarkIPTree_ReplaceWorst_AtCapacity(b *testing.B) {
	capacity := 1000
	tree := NewIPTree(capacity)
	for i := range capacity {
		tree.Insert(&types.Resolver{
			IP:      fmt.Sprintf("1.1.1.%d", i),
			Latency: time.Duration(i) * time.Millisecond,
		})
	}

	newRes := &types.Resolver{
		IP:      "2.2.2.2",
		Latency: 1 * time.Nanosecond,
	}

	for b.Loop() {
		tree.ReplaceWorst(newRes)
	}
}

func BenchmarkIPTree_SearchRange(b *testing.B) {
	tree := NewIPTree(10000)
	for i := range 255 {
		tree.Insert(&types.Resolver{
			IP:      fmt.Sprintf("192.168.1.%d", i),
			Latency: time.Duration(i) * time.Millisecond,
		})
	}

	for b.Loop() {
		_ = tree.SearchRange("192.168.1.0/24")
	}
}

func BenchmarkIPTree_GetAllSortedByLatency(b *testing.B) {
	tree := NewIPTree(1000)
	for i := range 1000 {
		tree.Insert(&types.Resolver{
			IP:      fmt.Sprintf("10.0.0.%d", i),
			Latency: time.Duration(1000-i) * time.Millisecond,
		})
	}

	for b.Loop() {
		_ = tree.GetAllSortedByLatency()
	}
}
