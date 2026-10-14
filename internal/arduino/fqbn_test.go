package arduino

import "testing"

func TestFQBN(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"name=Lib\narchitectures=esp32\n", "esp32:esp32:esp32"},
		{"architectures=esp32,avr\n", "esp32:esp32:esp32"},
		{"architectures=avr\n", "arduino:avr:uno"},
		{"architectures=*\n", "arduino:avr:uno"},
		{"name=Lib\n", "arduino:avr:uno"},
		{"architectures=samd\n", "arduino:avr:uno"},
	}
	for _, tc := range cases {
		if got := FQBN(tc.in); got != tc.want {
			t.Fatalf("in=%q got=%s want=%s", tc.in, got, tc.want)
		}
	}
}
