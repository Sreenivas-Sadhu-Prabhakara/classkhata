package core

import "testing"

func TestMonthsFromTo(t *testing.T) {
	got := MonthsFromTo("2026-03", "2026-07")
	want := []string{"2026-03", "2026-04", "2026-05", "2026-06", "2026-07"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if got := MonthsFromTo("2025-11", "2026-02"); len(got) != 4 || got[0] != "2025-11" || got[3] != "2026-02" {
		t.Fatalf("year rollover: got %v", got)
	}
	if got := MonthsFromTo("2026-08", "2026-07"); got != nil {
		t.Fatalf("from after to should be empty, got %v", got)
	}
	if got := MonthsFromTo("garbage", "2026-07"); got != nil {
		t.Fatalf("malformed month should be empty, got %v", got)
	}
}

func TestMissingDuesIdempotent(t *testing.T) {
	enrollments := []Enrollment{
		{ID: 1, BatchID: 10, JoinedMonth: "2026-03"},
		{ID: 2, BatchID: 10, JoinedMonth: "2026-06"},
	}
	fee := func(int) (int64, bool) { return 2500, true }

	first := MissingDues(enrollments, fee, nil, "2026-07")
	if len(first) != 5+2 {
		t.Fatalf("first generation: got %d dues, want 7", len(first))
	}
	second := MissingDues(enrollments, fee, first, "2026-07")
	if len(second) != 0 {
		t.Fatalf("regeneration must add nothing, got %d", len(second))
	}
}

func TestStoreEnsureDuesIdempotent(t *testing.T) {
	s, _ := NewStore("")
	b, err := s.CreateBatch(Batch{Name: "Physics XI", Subject: "Physics", Days: []string{"Mon"}, StartTime: "18:00", EndTime: "19:30", MonthlyFee: 2500})
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.CreateStudent(Student{Name: "Aarav Sharma", ParentName: "Rajesh Sharma", ParentPhone: "9811042001"})
	if err != nil {
		t.Fatal(err)
	}
	_, added, err := s.CreateEnrollment(st.ID, b.ID, "2026-03")
	if err != nil {
		t.Fatal(err)
	}
	if added != 5 {
		t.Fatalf("enrollment should generate 5 dues (Mar–Jul), got %d", added)
	}
	for i := 0; i < 3; i++ {
		if extra := s.EnsureDues(); extra != 0 {
			t.Fatalf("EnsureDues run %d added %d dues; regeneration must never duplicate", i+1, extra)
		}
	}
	if rows := s.DueRows(); len(rows) != 5 {
		t.Fatalf("ledger has %d rows, want 5", len(rows))
	}
}

func TestPartialPaymentOutstanding(t *testing.T) {
	due := Due{ID: 1, EnrollmentID: 7, Month: "2026-07", Amount: 2500}
	var payments []Payment

	if got := OutstandingForDue(due, payments); got != 2500 {
		t.Fatalf("unpaid due outstanding = %d, want 2500", got)
	}
	payments = append(payments, Payment{EnrollmentID: 7, Month: "2026-07", Amount: 1000})
	if got := OutstandingForDue(due, payments); got != 1500 {
		t.Fatalf("after ₹1,000 partial payment outstanding = %d, want 1500", got)
	}
	payments = append(payments, Payment{EnrollmentID: 7, Month: "2026-07", Amount: 800})
	if got := OutstandingForDue(due, payments); got != 700 {
		t.Fatalf("after second partial payment outstanding = %d, want 700", got)
	}
	// Overpayment never goes negative.
	payments = append(payments, Payment{EnrollmentID: 7, Month: "2026-07", Amount: 5000})
	if got := OutstandingForDue(due, payments); got != 0 {
		t.Fatalf("overpaid due outstanding = %d, want 0", got)
	}
	// Payments for other months or enrollments do not leak in.
	other := Due{ID: 2, EnrollmentID: 7, Month: "2026-06", Amount: 2500}
	if got := OutstandingForDue(other, payments); got != 2500 {
		t.Fatalf("other month outstanding = %d, want 2500", got)
	}
	if got := TotalOutstanding([]Due{due, other}, payments); got != 2500 {
		t.Fatalf("total outstanding = %d, want 2500", got)
	}
}
