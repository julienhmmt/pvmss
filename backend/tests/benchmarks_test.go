// Package tests provides benchmarking infrastructure for PVMSS.
// Run benchmarks with: go test -bench=. -benchmem ./tests/
package tests

import (
	"testing"
	"time"

	"pvmss/utils"
)

// BenchmarkCache measures cache performance.
func BenchmarkCache(b *testing.B) {
	cache := utils.CacheWith[string, int](time.Minute, 1000)
	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Set("key", i)
		}
	})
	b.Run("Get", func(b *testing.B) {
		cache.Set("key", 42)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get("key")
		}
	})
	b.Run("GetOrSet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.GetOrSet("key", func() int { return 42 })
		}
	})
}

// BenchmarkFilter measures slice filter performance.
func BenchmarkFilter(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = i
		}
		b.Run("size="+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				utils.Filter(slice, func(n int) bool { return n%2 == 0 })
			}
		})
	}
}

// BenchmarkMapSlice measures slice map performance.
func BenchmarkMapSlice(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = i
		}
		b.Run("size="+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				utils.MapSlice(slice, func(n int) int { return n * 2 })
			}
		})
	}
}

// BenchmarkReduce measures slice reduce performance.
func BenchmarkReduce(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = i
		}
		b.Run("size="+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				utils.Reduce(slice, 0, func(acc, n int) int { return acc + n })
			}
		})
	}
}

// BenchmarkUnique measures slice unique performance.
func BenchmarkUnique(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = i % (size / 2)
		}
		b.Run("size="+string(rune(size)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				utils.Unique(slice)
			}
		})
	}
}

// BenchmarkOptional measures Optional type performance.
func BenchmarkOptional(b *testing.B) {
	b.Run("Some", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			opt := utils.Some(42)
			_ = opt.IsPresent()
		}
	})
	b.Run("None", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			opt := utils.None[int]()
			_ = opt.IsPresent()
		}
	})
	b.Run("GetOrDefault", func(b *testing.B) {
		opt := utils.Some(42)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = opt.GetOrDefault(0)
		}
	})
}

// BenchmarkResult measures Result type performance.
func BenchmarkResult(b *testing.B) {
	b.Run("Ok", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r := utils.Ok(42)
			_ = r.IsOk()
		}
	})
	b.Run("UnwrapOr", func(b *testing.B) {
		r := utils.Ok(42)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = r.UnwrapOr(0)
		}
	})
}
