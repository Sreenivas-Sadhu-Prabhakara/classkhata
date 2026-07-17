package core

import (
	"fmt"
	"time"
)

// MonthsFromTo returns the inclusive list of "YYYY-MM" months between from
// and to. Empty when either month is malformed or from is after to.
func MonthsFromTo(from, to string) []string {
	fy, fm, ok1 := parseMonth(from)
	ty, tm, ok2 := parseMonth(to)
	if !ok1 || !ok2 {
		return nil
	}
	var out []string
	for y, m := fy, fm; y < ty || (y == ty && m <= tm); {
		out = append(out, fmt.Sprintf("%04d-%02d", y, m))
		m++
		if m > 12 {
			m = 1
			y++
		}
	}
	return out
}

func parseMonth(s string) (year, month int, ok bool) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return 0, 0, false
	}
	return t.Year(), int(t.Month()), true
}

// MonthLabel renders "2026-05" as "May 2026".
func MonthLabel(m string) string {
	t, err := time.Parse("2006-01", m)
	if err != nil {
		return m
	}
	return t.Format("Jan 2006")
}

// MissingDues computes the due rows that should exist but do not yet, for
// every enrollment from its joinedMonth through currentMonth. It is pure and
// idempotent: feeding its own output back in as existing dues yields nothing.
func MissingDues(enrollments []Enrollment, feeOf func(batchID int) (int64, bool), existing []Due, currentMonth string) []Due {
	have := make(map[string]bool, len(existing))
	for _, d := range existing {
		have[dueKey(d.EnrollmentID, d.Month)] = true
	}
	var out []Due
	for _, e := range enrollments {
		amount, ok := feeOf(e.BatchID)
		if !ok {
			continue
		}
		for _, m := range MonthsFromTo(e.JoinedMonth, currentMonth) {
			key := dueKey(e.ID, m)
			if !have[key] {
				have[key] = true
				out = append(out, Due{EnrollmentID: e.ID, Month: m, Amount: amount})
			}
		}
	}
	return out
}

func dueKey(enrollmentID int, month string) string {
	return fmt.Sprintf("%d|%s", enrollmentID, month)
}

// PaidForMonth sums payments recorded against one enrollment-month.
func PaidForMonth(payments []Payment, enrollmentID int, month string) int64 {
	var total int64
	for _, p := range payments {
		if p.EnrollmentID == enrollmentID && p.Month == month {
			total += p.Amount
		}
	}
	return total
}

// OutstandingForDue is due amount minus payments, floored at zero — an
// overpaid month never produces a negative balance.
func OutstandingForDue(d Due, payments []Payment) int64 {
	out := d.Amount - PaidForMonth(payments, d.EnrollmentID, d.Month)
	if out < 0 {
		return 0
	}
	return out
}

// TotalOutstanding sums the outstanding of every due.
func TotalOutstanding(dues []Due, payments []Payment) int64 {
	var total int64
	for _, d := range dues {
		total += OutstandingForDue(d, payments)
	}
	return total
}
