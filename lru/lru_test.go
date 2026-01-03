package lru_test

import (
	"reflect"
	"testing"

	"github.com/serroba/cache/lru"
)

func TestLRUCache_Get(t *testing.T) {
	type args[K comparable] struct {
		key K
	}

	type want[V any] struct {
		value V
		ok    bool
	}

	type testCase[K comparable, V any] struct {
		name string
		c    *lru.Cache[K, V]
		args args[K]
		want want[V]
	}

	tests := []testCase[string, int]{
		{
			name: "Test on empty cache",
			c:    lru.New[string, int](5),
			args: args[string]{key: "some"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.c.Get(tt.args.key)
			if !reflect.DeepEqual(got, tt.want.value) {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}

			if got1 != tt.want.ok {
				t.Errorf("Get() got1 = %v, want %v", got1, tt.want.value)
			}
		})
	}
}
