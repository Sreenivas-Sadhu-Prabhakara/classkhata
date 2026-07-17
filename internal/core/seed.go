package core

import (
	"strconv"
	"time"
)

// SeedDemo replaces the store contents with the deterministic demo dataset:
// 2 batches, 14 students, enrollments spread over the 4 months up to the
// anchor date, a mix of paid / partial / overdue fees, and this week's
// attendance registers (absences generate outbox alerts). Calling it twice
// yields byte-identical stores.
func (s *Store) SeedDemo(p Provider) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = data{}

	b1 := Batch{ID: s.nextID(), Name: "Physics XI", Subject: "Physics", Days: []string{"Mon", "Wed", "Fri"}, StartTime: "18:00", EndTime: "19:30", MonthlyFee: 2500}
	b2 := Batch{ID: s.nextID(), Name: "Maths X", Subject: "Mathematics", Days: []string{"Tue", "Thu", "Sat"}, StartTime: "17:00", EndTime: "18:30", MonthlyFee: 2000}
	s.d.Batches = append(s.d.Batches, b1, b2)

	seedStudents := []struct{ name, parent, phone string }{
		{"Aarav Sharma", "Rajesh Sharma", "+91-9811042001"},
		{"Ananya Iyer", "Suresh Iyer", "+91-9822153002"},
		{"Rohan Verma", "Manoj Verma", "+91-9833264003"},
		{"Priya Nair", "Deepa Nair", "+91-9844375004"},
		{"Kabir Singh", "Harpreet Singh", "+91-9855486005"},
		{"Sneha Kulkarni", "Anil Kulkarni", "+91-9866597006"},
		{"Arjun Reddy", "Prakash Reddy", "+91-9877608007"},
		{"Ishita Banerjee", "Sourav Banerjee", "+91-9888719008"},
		{"Vivaan Gupta", "Nitin Gupta", "+91-9899820009"},
		{"Diya Patel", "Hetal Patel", "+91-9810931010"},
		{"Aditya Joshi", "Mohan Joshi", "+91-9821042011"},
		{"Meera Krishnan", "Venkat Krishnan", "+91-9832153012"},
		{"Sahil Khan", "Imran Khan", "+91-9843264013"},
		{"Tanvi Desai", "Rakesh Desai", "+91-9854375014"},
	}
	students := make([]Student, 0, len(seedStudents))
	for _, ss := range seedStudents {
		st := Student{ID: s.nextID(), Name: ss.name, ParentName: ss.parent, ParentPhone: ss.phone}
		s.d.Students = append(s.d.Students, st)
		students = append(students, st)
	}

	// Students 0–7 study Physics XI; 6–13 study Maths X (6 & 7 take both).
	// Joining months cycle over the past four months up to the anchor.
	joinCycle := []string{"2026-04", "2026-05", "2026-06", "2026-07"}
	type membership struct {
		studentIdx int
		batch      Batch
	}
	var members []membership
	for i := 0; i < 8; i++ {
		members = append(members, membership{i, b1})
	}
	for i := 6; i < 14; i++ {
		members = append(members, membership{i, b2})
	}
	enrolls := make([]Enrollment, 0, len(members))
	for i, m := range members {
		e := Enrollment{ID: s.nextID(), StudentID: students[m.studentIdx].ID, BatchID: m.batch.ID, JoinedMonth: joinCycle[i%len(joinCycle)]}
		s.d.Enrollments = append(s.d.Enrollments, e)
		enrolls = append(enrolls, e)
	}

	s.ensureDuesLocked(AnchorMonth)

	// Payments: a third fully paid up, a third partial on the latest month,
	// a third overdue with only the first month cleared.
	modes := []string{"upi", "cash"}
	for i, e := range enrolls {
		months := MonthsFromTo(e.JoinedMonth, AnchorMonth)
		fee, _ := s.feeOfLocked(e.BatchID)
		switch i % 3 {
		case 0: // fully paid up
			for j, m := range months {
				s.d.Payments = append(s.d.Payments, Payment{
					ID: s.nextID(), EnrollmentID: e.ID, Month: m, Amount: fee,
					Mode: modes[(i+j)%2], Date: m + "-05",
				})
			}
		case 1: // paid until last month; latest month only 60% paid
			for j, m := range months {
				amount := fee
				if j == len(months)-1 {
					amount = fee * 3 / 5
				}
				s.d.Payments = append(s.d.Payments, Payment{
					ID: s.nextID(), EnrollmentID: e.ID, Month: m, Amount: amount,
					Mode: modes[(i+j)%2], Date: m + "-06",
				})
			}
		case 2: // overdue: only the joining month is cleared
			if len(months) > 0 {
				s.d.Payments = append(s.d.Payments, Payment{
					ID: s.nextID(), EnrollmentID: e.ID, Month: months[0], Amount: fee,
					Mode: modes[i%2], Date: months[0] + "-07",
				})
			}
		}
	}

	// This week's registers (anchor week: Mon 13 Jul – Fri 17 Jul 2026).
	// Absences are deterministic and flow through the normal alert path.
	schedule := []struct {
		batchID int
		dates   []string
	}{
		{b1.ID, []string{"2026-07-13", "2026-07-15", "2026-07-17"}},
		{b2.ID, []string{"2026-07-14", "2026-07-16"}},
	}
	for _, sc := range schedule {
		for _, date := range sc.dates {
			day, _ := strconv.Atoi(date[8:])
			marks := map[int]bool{}
			for _, e := range s.d.Enrollments {
				if e.BatchID != sc.batchID {
					continue
				}
				marks[e.StudentID] = (e.StudentID*7+day)%11 != 0
			}
			classEnd, _ := time.ParseInLocation("2006-01-02", date, IST)
			s.markAttendanceLocked(sc.batchID, date, marks, p, classEnd.Add(20*time.Hour))
		}
	}

	s.save()
	return map[string]int{
		"batches":     len(s.d.Batches),
		"students":    len(s.d.Students),
		"enrollments": len(s.d.Enrollments),
		"dues":        len(s.d.Dues),
		"payments":    len(s.d.Payments),
		"registers":   len(s.d.Attendance),
		"outbox":      len(s.d.Outbox),
	}
}
