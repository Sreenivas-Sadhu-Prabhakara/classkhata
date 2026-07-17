package core

import (
	"sort"
	"strings"
	"time"
)

// ---------- view models (what the API serves) ----------

// BatchView decorates a batch with display fields.
type BatchView struct {
	Batch
	StudentCount int    `json:"studentCount"`
	FeeFormatted string `json:"feeFormatted"`
	Schedule     string `json:"schedule"`
}

// EnrollSummary is one student's membership in one batch plus its balance.
type EnrollSummary struct {
	EnrollmentID         int    `json:"enrollmentId"`
	BatchID              int    `json:"batchId"`
	BatchName            string `json:"batchName"`
	JoinedMonth          string `json:"joinedMonth"`
	Outstanding          int64  `json:"outstanding"`
	OutstandingFormatted string `json:"outstandingFormatted"`
}

// StudentView decorates a student with balances and attendance.
type StudentView struct {
	Student
	Enrollments          []EnrollSummary `json:"enrollments"`
	AttendancePct        float64         `json:"attendancePct"`
	AttendanceMarked     int             `json:"attendanceMarked"`
	Outstanding          int64           `json:"outstanding"`
	OutstandingFormatted string          `json:"outstandingFormatted"`
}

// DueRow is one enrollment-month line in the fees ledger.
type DueRow struct {
	DueID                int    `json:"dueId"`
	EnrollmentID         int    `json:"enrollmentId"`
	StudentID            int    `json:"studentId"`
	StudentName          string `json:"studentName"`
	BatchName            string `json:"batchName"`
	Month                string `json:"month"`
	MonthLabel           string `json:"monthLabel"`
	Amount               int64  `json:"amount"`
	AmountFormatted      string `json:"amountFormatted"`
	Paid                 int64  `json:"paid"`
	PaidFormatted        string `json:"paidFormatted"`
	Outstanding          int64  `json:"outstanding"`
	OutstandingFormatted string `json:"outstandingFormatted"`
	Status               string `json:"status"` // paid | partial | unpaid
}

// PaymentView decorates a payment with names and formatting.
type PaymentView struct {
	Payment
	StudentName     string `json:"studentName"`
	BatchName       string `json:"batchName"`
	AmountFormatted string `json:"amountFormatted"`
	MonthLabel      string `json:"monthLabel"`
}

// GridStudent is one row of the attendance-marking grid.
type GridStudent struct {
	StudentID     int     `json:"studentId"`
	Name          string  `json:"name"`
	ParentPhone   string  `json:"parentPhone"`
	Present       *bool   `json:"present"` // nil = unmarked
	AttendancePct float64 `json:"attendancePct"`
	Marked        int     `json:"marked"`
}

// GridView is the attendance register for one batch-day.
type GridView struct {
	BatchID   int           `json:"batchId"`
	BatchName string        `json:"batchName"`
	Schedule  string        `json:"schedule"`
	Date      string        `json:"date"`
	Students  []GridStudent `json:"students"`
}

// Dashboard is the owner's morning glance.
type Dashboard struct {
	AnchorDate                string        `json:"anchorDate"`
	TodayLabel                string        `json:"todayLabel"`
	MonthLabel                string        `json:"monthLabel"`
	MonthCollections          int64         `json:"monthCollections"`
	MonthCollectionsFormatted string        `json:"monthCollectionsFormatted"`
	TotalOutstanding          int64         `json:"totalOutstanding"`
	TotalOutstandingFormatted string        `json:"totalOutstandingFormatted"`
	ActiveStudents            int           `json:"activeStudents"`
	TotalStudents             int           `json:"totalStudents"`
	OverdueEnrollments        int           `json:"overdueEnrollments"`
	WeekAttendancePct         float64       `json:"weekAttendancePct"`
	WeekPresent               int           `json:"weekPresent"`
	WeekTotal                 int           `json:"weekTotal"`
	TodaysBatches             []BatchView   `json:"todaysBatches"`
	RecentPayments            []PaymentView `json:"recentPayments"`
	OutboxCount               int           `json:"outboxCount"`
}

// ---------- builders ----------

func timeLabel(hm string) string {
	t, err := time.Parse("15:04", hm)
	if err != nil {
		return hm
	}
	return t.Format("3:04 PM")
}

func scheduleLabel(b Batch) string {
	return strings.Join(b.Days, " · ") + ", " + timeLabel(b.StartTime) + "–" + timeLabel(b.EndTime)
}

func (s *Store) batchViewLocked(b Batch) BatchView {
	count := 0
	for _, e := range s.d.Enrollments {
		if e.BatchID == b.ID {
			count++
		}
	}
	return BatchView{
		Batch:        b,
		StudentCount: count,
		FeeFormatted: FormatINR(b.MonthlyFee) + "/mo",
		Schedule:     scheduleLabel(b),
	}
}

// BatchViews lists every batch with display decorations.
func (s *Store) BatchViews() []BatchView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]BatchView, 0, len(s.d.Batches))
	for _, b := range s.d.Batches {
		out = append(out, s.batchViewLocked(b))
	}
	return out
}

// BatchView returns one decorated batch.
func (s *Store) BatchView(id int) (BatchView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batchByID(id)
	if !ok {
		return BatchView{}, false
	}
	return s.batchViewLocked(b), true
}

func (s *Store) enrollOutstandingLocked(enrollmentID int) int64 {
	var total int64
	for _, d := range s.d.Dues {
		if d.EnrollmentID == enrollmentID {
			total += OutstandingForDue(d, s.d.Payments)
		}
	}
	return total
}

// StudentViews lists every student with balances and attendance. Dues are
// regenerated first so balances are always current.
func (s *Store) StudentViews() []StudentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ensureDuesLocked(AnchorMonth) > 0 {
		s.save()
	}
	out := make([]StudentView, 0, len(s.d.Students))
	for _, st := range s.d.Students {
		v := StudentView{Student: st, Enrollments: []EnrollSummary{}}
		for _, e := range s.d.Enrollments {
			if e.StudentID != st.ID {
				continue
			}
			b, _ := s.batchByID(e.BatchID)
			o := s.enrollOutstandingLocked(e.ID)
			v.Enrollments = append(v.Enrollments, EnrollSummary{
				EnrollmentID:         e.ID,
				BatchID:              e.BatchID,
				BatchName:            b.Name,
				JoinedMonth:          e.JoinedMonth,
				Outstanding:          o,
				OutstandingFormatted: FormatINR(o),
			})
			v.Outstanding += o
		}
		v.OutstandingFormatted = FormatINR(v.Outstanding)
		v.AttendancePct = AttendancePercent(s.d.Attendance, st.ID, 0)
		_, v.AttendanceMarked = AttendanceStats(s.d.Attendance, st.ID, 0)
		out = append(out, v)
	}
	return out
}

// StudentView returns one decorated student.
func (s *Store) StudentView(id int) (StudentView, bool) {
	for _, v := range s.StudentViews() {
		if v.ID == id {
			return v, true
		}
	}
	return StudentView{}, false
}

// Enrollments returns raw enrollment rows.
func (s *Store) Enrollments() []Enrollment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Enrollment(nil), s.d.Enrollments...)
}

// DueRows lists the full fees ledger sorted by student then month; dues are
// regenerated first.
func (s *Store) DueRows() []DueRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ensureDuesLocked(AnchorMonth) > 0 {
		s.save()
	}
	out := make([]DueRow, 0, len(s.d.Dues))
	for _, d := range s.d.Dues {
		e, ok := s.enrollmentByID(d.EnrollmentID)
		if !ok {
			continue
		}
		st, _ := s.studentByID(e.StudentID)
		b, _ := s.batchByID(e.BatchID)
		paid := PaidForMonth(s.d.Payments, d.EnrollmentID, d.Month)
		outAmt := OutstandingForDue(d, s.d.Payments)
		status := "unpaid"
		switch {
		case outAmt == 0:
			status = "paid"
		case paid > 0:
			status = "partial"
		}
		out = append(out, DueRow{
			DueID:                d.ID,
			EnrollmentID:         d.EnrollmentID,
			StudentID:            st.ID,
			StudentName:          st.Name,
			BatchName:            b.Name,
			Month:                d.Month,
			MonthLabel:           MonthLabel(d.Month),
			Amount:               d.Amount,
			AmountFormatted:      FormatINR(d.Amount),
			Paid:                 paid,
			PaidFormatted:        FormatINR(paid),
			Outstanding:          outAmt,
			OutstandingFormatted: FormatINR(outAmt),
			Status:               status,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StudentName != out[j].StudentName {
			return out[i].StudentName < out[j].StudentName
		}
		if out[i].BatchName != out[j].BatchName {
			return out[i].BatchName < out[j].BatchName
		}
		return out[i].Month < out[j].Month
	})
	return out
}

func (s *Store) paymentViewLocked(p Payment) PaymentView {
	v := PaymentView{Payment: p, AmountFormatted: FormatINR(p.Amount), MonthLabel: MonthLabel(p.Month)}
	if e, ok := s.enrollmentByID(p.EnrollmentID); ok {
		if st, ok := s.studentByID(e.StudentID); ok {
			v.StudentName = st.Name
		}
		if b, ok := s.batchByID(e.BatchID); ok {
			v.BatchName = b.Name
		}
	}
	return v
}

// PaymentViews lists payments, newest first.
func (s *Store) PaymentViews() []PaymentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PaymentView, 0, len(s.d.Payments))
	for _, p := range s.d.Payments {
		out = append(out, s.paymentViewLocked(p))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Payment.ID > out[j].Payment.ID
	})
	return out
}

// AttendanceGrid builds the register for one batch-day, existing marks
// included.
func (s *Store) AttendanceGrid(batchID int, date string) (GridView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batchByID(batchID)
	if !ok {
		return GridView{}, ErrNotFound
	}
	var rec *AttendanceRecord
	for i := range s.d.Attendance {
		if s.d.Attendance[i].BatchID == batchID && s.d.Attendance[i].Date == date {
			rec = &s.d.Attendance[i]
			break
		}
	}
	grid := GridView{BatchID: batchID, BatchName: b.Name, Schedule: scheduleLabel(b), Date: date, Students: []GridStudent{}}
	for _, e := range s.d.Enrollments {
		if e.BatchID != batchID {
			continue
		}
		st, ok := s.studentByID(e.StudentID)
		if !ok {
			continue
		}
		gs := GridStudent{
			StudentID:     st.ID,
			Name:          st.Name,
			ParentPhone:   st.ParentPhone,
			AttendancePct: AttendancePercent(s.d.Attendance, st.ID, batchID),
		}
		_, gs.Marked = AttendanceStats(s.d.Attendance, st.ID, batchID)
		if rec != nil {
			if p, marked := rec.Marks[st.ID]; marked {
				val := p
				gs.Present = &val
			}
		}
		grid.Students = append(grid.Students, gs)
	}
	sort.SliceStable(grid.Students, func(i, j int) bool { return grid.Students[i].Name < grid.Students[j].Name })
	return grid, nil
}

// OutboxList returns messages newest-first.
func (s *Store) OutboxList() []OutboxMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OutboxMessage, len(s.d.Outbox))
	for i, m := range s.d.Outbox {
		out[len(s.d.Outbox)-1-i] = m
	}
	return out
}

// DashboardView assembles the anchor-day overview; dues are regenerated
// first so outstanding figures are current.
func (s *Store) DashboardView() Dashboard {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ensureDuesLocked(AnchorMonth) > 0 {
		s.save()
	}

	anchor, _ := time.Parse("2006-01-02", AnchorDate)
	d := Dashboard{
		AnchorDate:     AnchorDate,
		TodayLabel:     anchor.Format("Monday, 2 January 2006"),
		MonthLabel:     anchor.Format("January 2006"),
		TodaysBatches:  []BatchView{},
		RecentPayments: []PaymentView{},
		OutboxCount:    len(s.d.Outbox),
		TotalStudents:  len(s.d.Students),
	}

	for _, p := range s.d.Payments {
		if strings.HasPrefix(p.Date, AnchorMonth) {
			d.MonthCollections += p.Amount
		}
	}
	d.MonthCollectionsFormatted = FormatINR(d.MonthCollections)

	d.TotalOutstanding = TotalOutstanding(s.d.Dues, s.d.Payments)
	d.TotalOutstandingFormatted = FormatINR(d.TotalOutstanding)

	active := map[int]bool{}
	for _, e := range s.d.Enrollments {
		active[e.StudentID] = true
		if s.enrollOutstandingLocked(e.ID) > 0 {
			d.OverdueEnrollments++
		}
	}
	d.ActiveStudents = len(active)

	weekday := anchor.Format("Mon")
	for _, b := range s.d.Batches {
		for _, day := range b.Days {
			if day == weekday {
				d.TodaysBatches = append(d.TodaysBatches, s.batchViewLocked(b))
				break
			}
		}
	}

	monIdx := (int(anchor.Weekday()) + 6) % 7
	weekStart := anchor.AddDate(0, 0, -monIdx).Format("2006-01-02")
	weekEnd := anchor.AddDate(0, 0, 6-monIdx).Format("2006-01-02")
	for _, r := range s.d.Attendance {
		if r.Date < weekStart || r.Date > weekEnd {
			continue
		}
		for _, present := range r.Marks {
			d.WeekTotal++
			if present {
				d.WeekPresent++
			}
		}
	}
	if d.WeekTotal > 0 {
		d.WeekAttendancePct = float64(int(float64(d.WeekPresent)/float64(d.WeekTotal)*1000+0.5)) / 10
	}

	views := make([]PaymentView, 0, len(s.d.Payments))
	for _, p := range s.d.Payments {
		views = append(views, s.paymentViewLocked(p))
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].Date != views[j].Date {
			return views[i].Date > views[j].Date
		}
		return views[i].Payment.ID > views[j].Payment.ID
	})
	if len(views) > 6 {
		views = views[:6]
	}
	d.RecentPayments = views

	return d
}
