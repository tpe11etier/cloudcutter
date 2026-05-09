package config

import (
	"testing"
)

func TestExpandEnv(t *testing.T) {
	t.Setenv("CC_TEST_A", "alpha")
	t.Setenv("CC_TEST_EMPTY", "")

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no vars", "plain string", "plain string"},
		{"basic var", "x=${CC_TEST_A}", "x=alpha"},
		{"unset, no default", "x=${CC_TEST_NOPE}", "x="},
		{"unset, with default", "x=${CC_TEST_NOPE:-fallback}", "x=fallback"},
		{"empty value treats as set", "x=${CC_TEST_EMPTY:-fallback}", "x=fallback"},
		{"two vars in one string", "${CC_TEST_A}-${CC_TEST_NOPE:-z}", "alpha-z"},
		{"adjacent literals around var", "[${CC_TEST_A}]", "[alpha]"},
		{"escaped dollar passes through", "$$VAR", "$$VAR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpandEnv(c.in)
			if got != c.want {
				t.Errorf("ExpandEnv(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
