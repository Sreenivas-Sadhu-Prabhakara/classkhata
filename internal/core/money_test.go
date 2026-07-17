package core

import "testing"

func TestFormatINR(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "₹0"},
		{250, "₹250"},
		{12500, "₹12,500"},
		{99999, "₹99,999"},
		{100000, "₹1 L"},
		{365000 * 10, "₹36.5 L"},
		{1234567, "₹12.3 L"},
		{10000000, "₹1 Cr"},
		{12000000, "₹1.2 Cr"},
		{-12500, "-₹12,500"},
	}
	for _, c := range cases {
		if got := FormatINR(c.in); got != c.want {
			t.Errorf("FormatINR(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGroupINR(t *testing.T) {
	if got := groupINR(1234567); got != "12,34,567" {
		t.Errorf("groupINR(1234567) = %q, want 12,34,567", got)
	}
}
