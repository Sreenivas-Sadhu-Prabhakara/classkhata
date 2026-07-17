package core

import (
	"strconv"
	"strings"
)

// FormatINR renders rupee amounts the way an Indian institute owner reads
// them: ₹1.2 Cr, ₹36.5 L, ₹12,500.
func FormatINR(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	switch {
	case amount >= 1_00_00_000:
		return sign + "₹" + trimZero(float64(amount)/1e7) + " Cr"
	case amount >= 1_00_000:
		return sign + "₹" + trimZero(float64(amount)/1e5) + " L"
	default:
		return sign + "₹" + groupINR(amount)
	}
}

func trimZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

// groupINR applies Indian digit grouping: 1234567 -> "12,34,567".
func groupINR(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	head, tail := s[:len(s)-3], s[len(s)-3:]
	var parts []string
	for len(head) > 2 {
		parts = append([]string{head[len(head)-2:]}, parts...)
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append([]string{head}, parts...)
	}
	return strings.Join(parts, ",") + "," + tail
}
