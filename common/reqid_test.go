package common

import (
	"testing"
)

func BenchmarkGetReqID(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GetReqID()
		}
	})
}

func BenchmarkGetReqIDParallel(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GetReqID()
		}
	})
}

func TestGetReqID(t *testing.T) {
	t.Log(GetReqID())
}
