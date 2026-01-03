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

func TestLRUCache_Set(t *testing.T) {
	type args[K comparable, V any] struct {
		key   K
		value V
	}

	type want[V any] struct {
		value V
		ok    bool
	}

	type testCase[K comparable, V any] struct {
		name string
		c    *lru.Cache[K, V]
		args args[K, V]
		want want[V]
	}

	tests := []testCase[string, int]{
		{
			name: "Set and retrieve value",
			c:    lru.New[string, int](5),
			args: args[string, int]{key: "foo", value: 42},
			want: want[int]{value: 42, ok: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.Set(tt.args.key, tt.args.value)

			got, got1 := tt.c.Get(tt.args.key)
			if !reflect.DeepEqual(got, tt.want.value) {
				t.Errorf("Set() got = %v, want %v", got, tt.want.value)
			}

			if got1 != tt.want.ok {
				t.Errorf("Set() got1 = %v, want %v", got1, tt.want.ok)
			}
		})
	}
}
