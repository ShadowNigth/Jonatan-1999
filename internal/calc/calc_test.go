package calc

import (
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"1+1", 2},
		{"2*3+4", 10},
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"10/4", 2.5},
		{"2^10", 1024},
		{"-5 + 3", -2},
		{"10 % 3", 1},
		{"sqrt(16)", 4},
		{"round(2.6)", 3},
		{"2*pi", 2 * math.Pi},
		{"1e3 + 1", 1001},
		{"2**3", 8},
		{"100/8*2", 25},
	}
	for _, c := range cases {
		got, err := Eval(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("%s = %v, esperaba %v", c.expr, got, c.want)
		}
	}
}

func TestErrores(t *testing.T) {
	for _, expr := range []string{"", "1+", "1/0", "(1+2", "foo(1)", "2**"} {
		if _, err := Eval(expr); err == nil {
			t.Fatalf("%q debía fallar", expr)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[float64]string{
		2:    "2",
		2.5:  "2.5",
		1024: "1024",
		0.1:  "0.1",
		-3:   "-3",
	}
	for v, want := range cases {
		if got := Format(v); got != want {
			t.Fatalf("Format(%v) = %q, esperaba %q", v, got, want)
		}
	}
}
