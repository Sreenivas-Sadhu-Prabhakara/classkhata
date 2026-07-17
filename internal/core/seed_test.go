package core

import (
	"bytes"
	"testing"
)

func TestSeedDeterminism(t *testing.T) {
	s1, _ := NewStore("")
	s2, _ := NewStore("")
	c1 := s1.SeedDemo(MockWhatsApp{})
	c2 := s2.SeedDemo(MockWhatsApp{})

	for k, v := range c1 {
		if c2[k] != v {
			t.Fatalf("seed counts differ for %s: %d vs %d", k, v, c2[k])
		}
	}
	j1, err := s1.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := s2.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(j1, j2) {
		t.Fatal("two demo seeds produced different stores; seed must be deterministic")
	}
	// Reseeding the same store is also a clean reset.
	s1.SeedDemo(MockWhatsApp{})
	j3, _ := s1.ExportJSON()
	if !bytes.Equal(j1, j3) {
		t.Fatal("reseeding an already-seeded store diverged")
	}
}

func TestSeedShape(t *testing.T) {
	s, _ := NewStore("")
	counts := s.SeedDemo(MockWhatsApp{})

	if counts["batches"] != 2 {
		t.Errorf("batches = %d, want 2", counts["batches"])
	}
	if counts["students"] != 14 {
		t.Errorf("students = %d, want 14", counts["students"])
	}
	if counts["enrollments"] != 16 {
		t.Errorf("enrollments = %d, want 16", counts["enrollments"])
	}
	// Joining months Apr–Jul cycling over 16 enrollments: 4 each of 4,3,2,1 dues.
	if counts["dues"] != 40 {
		t.Errorf("dues = %d, want 40", counts["dues"])
	}
	if counts["outbox"] == 0 {
		t.Error("seed should produce at least one absence alert in the outbox")
	}

	d := s.DashboardView()
	if d.TotalOutstanding <= 0 {
		t.Error("demo data must include unpaid dues")
	}
	if d.MonthCollections <= 0 {
		t.Error("demo data must include payments this month")
	}
	if d.ActiveStudents != 14 {
		t.Errorf("active students = %d, want 14", d.ActiveStudents)
	}
	if len(d.TodaysBatches) != 1 || d.TodaysBatches[0].Name != "Physics XI" {
		t.Errorf("anchor Friday should have exactly Physics XI, got %+v", d.TodaysBatches)
	}
	if d.WeekTotal == 0 || d.WeekAttendancePct <= 0 {
		t.Errorf("week attendance should be recorded, got %d marks %.1f%%", d.WeekTotal, d.WeekAttendancePct)
	}
}
