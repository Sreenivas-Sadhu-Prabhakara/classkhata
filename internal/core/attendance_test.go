package core

import (
	"testing"
	"time"
)

func TestAttendancePercent(t *testing.T) {
	records := []AttendanceRecord{
		{BatchID: 1, Date: "2026-07-13", Marks: map[int]bool{5: true, 6: false}},
		{BatchID: 1, Date: "2026-07-15", Marks: map[int]bool{5: true, 6: true}},
		{BatchID: 1, Date: "2026-07-17", Marks: map[int]bool{5: false, 6: true}},
		{BatchID: 2, Date: "2026-07-14", Marks: map[int]bool{5: true}},
	}
	if got := AttendancePercent(records, 5, 1); got != 66.7 {
		t.Errorf("student 5 batch 1 = %v, want 66.7", got)
	}
	if got := AttendancePercent(records, 5, 0); got != 75.0 {
		t.Errorf("student 5 all batches = %v, want 75.0", got)
	}
	if got := AttendancePercent(records, 6, 1); got != 66.7 {
		t.Errorf("student 6 = %v, want 66.7", got)
	}
	if got := AttendancePercent(records, 99, 0); got != 0 {
		t.Errorf("unmarked student = %v, want 0", got)
	}
}

func TestMarkAttendanceIdempotentAlerts(t *testing.T) {
	s, _ := NewStore("")
	b, _ := s.CreateBatch(Batch{Name: "Maths X", Subject: "Maths", Days: []string{"Tue"}, StartTime: "17:00", EndTime: "18:30", MonthlyFee: 2000})
	st, _ := s.CreateStudent(Student{Name: "Diya Patel", ParentName: "Hetal Patel", ParentPhone: "9810931010"})
	if _, _, err := s.CreateEnrollment(st.ID, b.ID, "2026-07"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 20, 0, 0, 0, IST)

	_, alerts, err := s.MarkAttendance(b.ID, "2026-07-14", map[int]bool{st.ID: false}, MockWhatsApp{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if alerts != 1 {
		t.Fatalf("first absent mark should alert once, got %d", alerts)
	}
	// Saving the identical grid again must not duplicate the alert.
	_, alerts, err = s.MarkAttendance(b.ID, "2026-07-14", map[int]bool{st.ID: false}, MockWhatsApp{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if alerts != 0 {
		t.Fatalf("re-saving same grid should alert zero times, got %d", alerts)
	}
	if got := len(s.OutboxList()); got != 1 {
		t.Fatalf("outbox has %d messages, want 1", got)
	}
	// Flipping to present then back to absent is a new absence.
	s.MarkAttendance(b.ID, "2026-07-14", map[int]bool{st.ID: true}, MockWhatsApp{}, now)
	_, alerts, _ = s.MarkAttendance(b.ID, "2026-07-14", map[int]bool{st.ID: false}, MockWhatsApp{}, now)
	if alerts != 1 {
		t.Fatalf("fresh absence after present should alert, got %d", alerts)
	}
}
