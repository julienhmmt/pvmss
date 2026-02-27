package utils

import (
	"errors"
	"testing"
	"time"
)

func TestOptional(t *testing.T) {
	tests := []struct {
		name      string
		opt       Optional[int]
		wantValue int
		wantOk    bool
	}{
		{
			name:      "Some value",
			opt:       Some(42),
			wantValue: 42,
			wantOk:    true,
		},
		{
			name:      "None value",
			opt:       None[int](),
			wantValue: 0,
			wantOk:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opt.IsPresent() != tt.wantOk {
				t.Errorf("IsPresent() = %v, want %v", tt.opt.IsPresent(), tt.wantOk)
			}
			value, ok := tt.opt.Get()
			if ok != tt.wantOk {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantOk)
			}
			if value != tt.wantValue {
				t.Errorf("Get() value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestOptionalGetOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		opt          Optional[string]
		defaultValue string
		want         string
	}{
		{
			name:         "present value",
			opt:          Some("hello"),
			defaultValue: "default",
			want:         "hello",
		},
		{
			name:         "absent value",
			opt:          None[string](),
			defaultValue: "default",
			want:         "default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opt.GetOrDefault(tt.defaultValue); got != tt.want {
				t.Errorf("GetOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResult(t *testing.T) {
	tests := []struct {
		name    string
		result  Result[int]
		wantOk  bool
		wantErr bool
	}{
		{
			name:    "Ok result",
			result:  Ok(42),
			wantOk:  true,
			wantErr: false,
		},
		{
			name:    "Err result",
			result:  Err[int](errors.New("error")),
			wantOk:  false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsOk() != tt.wantOk {
				t.Errorf("IsOk() = %v, want %v", tt.result.IsOk(), tt.wantOk)
			}
			if tt.result.IsErr() != tt.wantErr {
				t.Errorf("IsErr() = %v, want %v", tt.result.IsErr(), tt.wantErr)
			}
		})
	}
}

func TestCache(t *testing.T) {
	cache := CacheWith[string, int](time.Minute, 10)
	cache.Set("key1", 100)
	cache.Set("key2", 200)
	tests := []struct {
		name    string
		key     string
		wantVal int
		wantOk  bool
	}{
		{
			name:    "existing key",
			key:     "key1",
			wantVal: 100,
			wantOk:  true,
		},
		{
			name:    "another existing key",
			key:     "key2",
			wantVal: 200,
			wantOk:  true,
		},
		{
			name:    "non-existing key",
			key:     "key3",
			wantVal: 0,
			wantOk:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := cache.Get(tt.key)
			if ok != tt.wantOk {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantOk)
			}
			if val != tt.wantVal {
				t.Errorf("Get() val = %v, want %v", val, tt.wantVal)
			}
		})
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := CacheWith[string, int](50*time.Millisecond, 10)
	cache.Set("key", 42)
	val, ok := cache.Get("key")
	if !ok || val != 42 {
		t.Error("Value should be present immediately after set")
	}
	time.Sleep(100 * time.Millisecond)
	_, ok = cache.Get("key")
	if ok {
		t.Error("Value should have expired")
	}
}

func TestCacheGetOrSet(t *testing.T) {
	cache := CacheWith[string, int](time.Minute, 10)
	callCount := 0
	fn := func() int {
		callCount++
		return 42
	}
	val1 := cache.GetOrSet("key", fn)
	if val1 != 42 {
		t.Errorf("GetOrSet() = %v, want 42", val1)
	}
	if callCount != 1 {
		t.Errorf("fn should be called once, got %d", callCount)
	}
	val2 := cache.GetOrSet("key", fn)
	if val2 != 42 {
		t.Errorf("GetOrSet() = %v, want 42", val2)
	}
	if callCount != 1 {
		t.Errorf("fn should not be called again, got %d", callCount)
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		slice     []int
		predicate func(int) bool
		want      []int
	}{
		{
			name:      "filter even numbers",
			slice:     []int{1, 2, 3, 4, 5, 6},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      []int{2, 4, 6},
		},
		{
			name:      "filter positive",
			slice:     []int{-1, 0, 1, 2},
			predicate: func(n int) bool { return n > 0 },
			want:      []int{1, 2},
		},
		{
			name:      "empty slice",
			slice:     []int{},
			predicate: func(n int) bool { return true },
			want:      []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.slice, tt.predicate)
			if len(got) != len(tt.want) {
				t.Errorf("Filter() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Filter()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMapSlice(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		fn    func(int) int
		want  []int
	}{
		{
			name:  "double values",
			slice: []int{1, 2, 3},
			fn:    func(n int) int { return n * 2 },
			want:  []int{2, 4, 6},
		},
		{
			name:  "square values",
			slice: []int{1, 2, 3},
			fn:    func(n int) int { return n * n },
			want:  []int{1, 4, 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapSlice(tt.slice, tt.fn)
			if len(got) != len(tt.want) {
				t.Errorf("MapSlice() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("MapSlice()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReduce(t *testing.T) {
	tests := []struct {
		name    string
		slice   []int
		initial int
		fn      func(int, int) int
		want    int
	}{
		{
			name:    "sum",
			slice:   []int{1, 2, 3, 4, 5},
			initial: 0,
			fn:      func(acc, n int) int { return acc + n },
			want:    15,
		},
		{
			name:    "product",
			slice:   []int{1, 2, 3, 4},
			initial: 1,
			fn:      func(acc, n int) int { return acc * n },
			want:    24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reduce(tt.slice, tt.initial, tt.fn); got != tt.want {
				t.Errorf("Reduce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name    string
		slice   []string
		element string
		want    bool
	}{
		{
			name:    "element present",
			slice:   []string{"a", "b", "c"},
			element: "b",
			want:    true,
		},
		{
			name:    "element absent",
			slice:   []string{"a", "b", "c"},
			element: "d",
			want:    false,
		},
		{
			name:    "empty slice",
			slice:   []string{},
			element: "a",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.slice, tt.element); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		want  []int
	}{
		{
			name:  "with duplicates",
			slice: []int{1, 2, 2, 3, 3, 3},
			want:  []int{1, 2, 3},
		},
		{
			name:  "no duplicates",
			slice: []int{1, 2, 3},
			want:  []int{1, 2, 3},
		},
		{
			name:  "empty slice",
			slice: []int{},
			want:  []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.slice)
			if len(got) != len(tt.want) {
				t.Errorf("Unique() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Unique()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFind(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	found := Find(slice, func(n int) bool { return n > 3 })
	if !found.IsPresent() {
		t.Error("Find() should find element > 3")
	}
	val, _ := found.Get()
	if val != 4 {
		t.Errorf("Find() = %v, want 4", val)
	}
	notFound := Find(slice, func(n int) bool { return n > 10 })
	if notFound.IsPresent() {
		t.Error("Find() should not find element > 10")
	}
}

func TestFirstLast(t *testing.T) {
	slice := []int{1, 2, 3}
	first := First(slice)
	if !first.IsPresent() {
		t.Error("First() should be present")
	}
	if val, _ := first.Get(); val != 1 {
		t.Errorf("First() = %v, want 1", val)
	}
	last := Last(slice)
	if !last.IsPresent() {
		t.Error("Last() should be present")
	}
	if val, _ := last.Get(); val != 3 {
		t.Errorf("Last() = %v, want 3", val)
	}
	emptyFirst := First([]int{})
	if emptyFirst.IsPresent() {
		t.Error("First() on empty slice should not be present")
	}
}

func TestPtr(t *testing.T) {
	val := 42
	ptr := Ptr(val)
	if *ptr != 42 {
		t.Errorf("Ptr() = %v, want 42", *ptr)
	}
}

func TestDeref(t *testing.T) {
	val := 42
	ptr := &val
	if Deref(ptr) != 42 {
		t.Errorf("Deref() = %v, want 42", Deref(ptr))
	}
	var nilPtr *int
	if Deref(nilPtr) != 0 {
		t.Errorf("Deref(nil) = %v, want 0", Deref(nilPtr))
	}
}

func TestDerefOr(t *testing.T) {
	val := 42
	ptr := &val
	if DerefOr(ptr, 0) != 42 {
		t.Errorf("DerefOr() = %v, want 42", DerefOr(ptr, 0))
	}
	var nilPtr *int
	if DerefOr(nilPtr, 99) != 99 {
		t.Errorf("DerefOr(nil) = %v, want 99", DerefOr(nilPtr, 99))
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "first non-empty",
			values: []string{"", "hello", "world"},
			want:   "hello",
		},
		{
			name:   "all empty",
			values: []string{"", "", ""},
			want:   "",
		},
		{
			name:   "first is non-empty",
			values: []string{"first", "second"},
			want:   "first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Coalesce(tt.values...); got != tt.want {
				t.Errorf("Coalesce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupBy(t *testing.T) {
	type person struct {
		name string
		age  int
	}
	people := []person{
		{name: "Alice", age: 30},
		{name: "Bob", age: 25},
		{name: "Charlie", age: 30},
	}
	grouped := GroupBy(people, func(p person) int { return p.age })
	if len(grouped[30]) != 2 {
		t.Errorf("GroupBy()[30] len = %v, want 2", len(grouped[30]))
	}
	if len(grouped[25]) != 1 {
		t.Errorf("GroupBy()[25] len = %v, want 1", len(grouped[25]))
	}
}

func TestKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := Keys(m)
	if len(keys) != 3 {
		t.Errorf("Keys() len = %v, want 3", len(keys))
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("Keys() contains invalid key %v", k)
		}
	}
}

func TestValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	values := Values(m)
	if len(values) != 3 {
		t.Errorf("Values() len = %v, want 3", len(values))
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	if sum != 6 {
		t.Errorf("Values() sum = %v, want 6", sum)
	}
}
