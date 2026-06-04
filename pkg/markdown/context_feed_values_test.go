package markdown

import "testing"

func TestStrVal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "string", in: "sample", want: "sample"},
		{name: "fallback", in: 42, want: "42"},
	}
	for _, tc := range cases {
		if got := strVal(tc.in); got != tc.want {
			t.Fatalf("%s: strVal(%v)=%q; want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestIntVal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want int
	}{
		{name: "nil", in: nil, want: 0},
		{name: "float64", in: 12.9, want: 12},
		{name: "int", in: 7, want: 7},
		{name: "int64", in: int64(8), want: 8},
		{name: "unsupported", in: "9", want: 0},
	}
	for _, tc := range cases {
		if got := intVal(tc.in); got != tc.want {
			t.Fatalf("%s: intVal(%v)=%d; want %d", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestFirstIntVal(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"zero":  0,
		"later": 42,
	}
	if got := firstIntVal(values, "missing", "zero", "later"); got != 42 {
		t.Fatalf("firstIntVal returned %d; want 42", got)
	}
	if got := firstIntVal(values, "missing", "zero"); got != 0 {
		t.Fatalf("firstIntVal without non-zero value returned %d; want 0", got)
	}
}

func TestInt64Val(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want int64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "float64", in: 12.9, want: 12},
		{name: "int64", in: int64(8), want: 8},
		{name: "int", in: 7, want: 7},
		{name: "unsupported", in: "9", want: 0},
	}
	for _, tc := range cases {
		if got := int64Val(tc.in); got != tc.want {
			t.Fatalf("%s: int64Val(%v)=%d; want %d", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestUintVal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want uint64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "float64", in: 12.9, want: 12},
		{name: "int", in: 7, want: 7},
		{name: "uint64", in: uint64(8), want: 8},
		{name: "int64", in: int64(9), want: 9},
		{name: "unsupported", in: "10", want: 0},
	}
	for _, tc := range cases {
		if got := uintVal(tc.in); got != tc.want {
			t.Fatalf("%s: uintVal(%v)=%d; want %d", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestUint32Val(t *testing.T) {
	t.Parallel()

	if got := uint32Val(uint64(11)); got != 11 {
		t.Fatalf("uint32Val returned %d; want 11", got)
	}
}

func TestFloat64Val(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want float64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "float64", in: 12.5, want: 12.5},
		{name: "int", in: 7, want: 7},
		{name: "unsupported", in: int64(8), want: 0},
	}
	for _, tc := range cases {
		if got := float64Val(tc.in); got != tc.want {
			t.Fatalf("%s: float64Val(%v)=%v; want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestBoolVal(t *testing.T) {
	t.Parallel()

	if got := boolVal(true); !got {
		t.Fatal("boolVal(true)=false; want true")
	}
	if got := boolVal("true"); got {
		t.Fatal(`boolVal("true")=true; want false`)
	}
	if got := boolVal(nil); got {
		t.Fatal("boolVal(nil)=true; want false")
	}
}

func TestToFloat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{name: "float64", in: 12.5, want: 12.5, ok: true},
		{name: "int", in: 7, want: 7, ok: true},
		{name: "int64", in: int64(8), want: 8, ok: true},
		{name: "uint64", in: uint64(9), want: 9, ok: true},
		{name: "unsupported", in: "10", want: 0, ok: false},
	}
	for _, tc := range cases {
		got, ok := toFloat(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%s: toFloat(%v)=(%v,%v); want (%v,%v)", tc.name, tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
