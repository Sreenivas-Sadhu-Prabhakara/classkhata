package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound marks lookups of entities that do not exist.
var ErrNotFound = errors.New("not found")

// IST is the timezone every ClassKhata timestamp is rendered in.
var IST = time.FixedZone("IST", 5*3600+30*60)

// Store is the in-memory database with JSON snapshot persistence. Every
// mutation happens under one mutex and is followed by a snapshot write.
type Store struct {
	mu   sync.Mutex
	path string
	d    data
}

type data struct {
	Batches     []Batch            `json:"batches"`
	Students    []Student          `json:"students"`
	Enrollments []Enrollment       `json:"enrollments"`
	Dues        []Due              `json:"dues"`
	Payments    []Payment          `json:"payments"`
	Attendance  []AttendanceRecord `json:"attendance"`
	Outbox      []OutboxMessage    `json:"outbox"`
	NextID      int                `json:"nextId"`
}

// NewStore loads the snapshot at path if one exists. An empty path disables
// persistence (used by tests).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.d); err != nil {
		return nil, fmt.Errorf("corrupt snapshot %s: %w", path, err)
	}
	return s, nil
}

// save writes the snapshot; callers must hold s.mu.
func (s *Store) save() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "classkhata: snapshot dir: %v\n", err)
		return
	}
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err == nil {
		err = os.WriteFile(s.path, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "classkhata: snapshot write: %v\n", err)
	}
}

func (s *Store) nextID() int {
	s.d.NextID++
	return s.d.NextID
}

// ExportJSON returns the full snapshot; used by tests to prove determinism.
func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.MarshalIndent(s.d, "", "  ")
}

// ---------- lookups (lock held) ----------

func (s *Store) batchByID(id int) (Batch, bool) {
	for _, b := range s.d.Batches {
		if b.ID == id {
			return b, true
		}
	}
	return Batch{}, false
}

func (s *Store) studentByID(id int) (Student, bool) {
	for _, st := range s.d.Students {
		if st.ID == id {
			return st, true
		}
	}
	return Student{}, false
}

func (s *Store) enrollmentByID(id int) (Enrollment, bool) {
	for _, e := range s.d.Enrollments {
		if e.ID == id {
			return e, true
		}
	}
	return Enrollment{}, false
}

func (s *Store) feeOfLocked(batchID int) (int64, bool) {
	b, ok := s.batchByID(batchID)
	if !ok {
		return 0, false
	}
	return b.MonthlyFee, true
}

// ---------- validation ----------

func validateBatch(b Batch) error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("batch name is required")
	}
	if b.MonthlyFee <= 0 {
		return fmt.Errorf("monthly fee must be greater than 0")
	}
	if len(b.Days) == 0 {
		return fmt.Errorf("pick at least one schedule day")
	}
	for _, d := range b.Days {
		if !validDay(d) {
			return fmt.Errorf("invalid day %q (use Mon…Sun)", d)
		}
	}
	for _, tm := range []string{b.StartTime, b.EndTime} {
		if _, err := time.Parse("15:04", tm); err != nil {
			return fmt.Errorf("times must be HH:MM in 24h format")
		}
	}
	return nil
}

func validDay(d string) bool {
	for _, v := range ValidDays {
		if v == d {
			return true
		}
	}
	return false
}

// NormalizePhone accepts common Indian mobile spellings and returns the
// canonical "+91-XXXXXXXXXX" form.
func NormalizePhone(p string) (string, error) {
	var digits strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) == 12 && strings.HasPrefix(d, "91") {
		d = d[2:]
	}
	if len(d) != 10 || d[0] < '6' {
		return "", fmt.Errorf("parent phone must be a 10-digit Indian mobile (+91)")
	}
	return "+91-" + d, nil
}

func validateStudent(st Student) (Student, error) {
	if strings.TrimSpace(st.Name) == "" {
		return st, fmt.Errorf("student name is required")
	}
	if strings.TrimSpace(st.ParentName) == "" {
		return st, fmt.Errorf("parent name is required")
	}
	phone, err := NormalizePhone(st.ParentPhone)
	if err != nil {
		return st, err
	}
	st.ParentPhone = phone
	return st, nil
}

// ---------- batches ----------

func (s *Store) CreateBatch(b Batch) (Batch, error) {
	if err := validateBatch(b); err != nil {
		return Batch{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b.ID = s.nextID()
	s.d.Batches = append(s.d.Batches, b)
	s.save()
	return b, nil
}

func (s *Store) UpdateBatch(id int, b Batch) (Batch, error) {
	if err := validateBatch(b); err != nil {
		return Batch{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.d.Batches {
		if s.d.Batches[i].ID == id {
			b.ID = id
			s.d.Batches[i] = b
			s.save()
			return b, nil
		}
	}
	return Batch{}, fmt.Errorf("batch %d: %w", id, ErrNotFound)
}

// DeleteBatch removes the batch and cascades to its enrollments, their dues
// and payments, and the batch's attendance registers. Outbox history stays.
func (s *Store) DeleteBatch(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.batchByID(id); !ok {
		return fmt.Errorf("batch %d: %w", id, ErrNotFound)
	}
	gone := map[int]bool{}
	for _, e := range s.d.Enrollments {
		if e.BatchID == id {
			gone[e.ID] = true
		}
	}
	s.d.Batches = filter(s.d.Batches, func(b Batch) bool { return b.ID != id })
	s.d.Enrollments = filter(s.d.Enrollments, func(e Enrollment) bool { return e.BatchID != id })
	s.d.Dues = filter(s.d.Dues, func(d Due) bool { return !gone[d.EnrollmentID] })
	s.d.Payments = filter(s.d.Payments, func(p Payment) bool { return !gone[p.EnrollmentID] })
	s.d.Attendance = filter(s.d.Attendance, func(r AttendanceRecord) bool { return r.BatchID != id })
	s.save()
	return nil
}

// ---------- students ----------

func (s *Store) CreateStudent(st Student) (Student, error) {
	st, err := validateStudent(st)
	if err != nil {
		return Student{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st.ID = s.nextID()
	s.d.Students = append(s.d.Students, st)
	s.save()
	return st, nil
}

func (s *Store) UpdateStudent(id int, st Student) (Student, error) {
	st, err := validateStudent(st)
	if err != nil {
		return Student{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.d.Students {
		if s.d.Students[i].ID == id {
			st.ID = id
			s.d.Students[i] = st
			s.save()
			return st, nil
		}
	}
	return Student{}, fmt.Errorf("student %d: %w", id, ErrNotFound)
}

// DeleteStudent removes the student, their enrollments, dues, payments and
// attendance marks. Outbox history stays.
func (s *Store) DeleteStudent(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.studentByID(id); !ok {
		return fmt.Errorf("student %d: %w", id, ErrNotFound)
	}
	gone := map[int]bool{}
	for _, e := range s.d.Enrollments {
		if e.StudentID == id {
			gone[e.ID] = true
		}
	}
	s.d.Students = filter(s.d.Students, func(st Student) bool { return st.ID != id })
	s.d.Enrollments = filter(s.d.Enrollments, func(e Enrollment) bool { return e.StudentID != id })
	s.d.Dues = filter(s.d.Dues, func(d Due) bool { return !gone[d.EnrollmentID] })
	s.d.Payments = filter(s.d.Payments, func(p Payment) bool { return !gone[p.EnrollmentID] })
	for i := range s.d.Attendance {
		delete(s.d.Attendance[i].Marks, id)
	}
	s.save()
	return nil
}

// ---------- enrollments ----------

// CreateEnrollment links a student to a batch and immediately generates the
// missing monthly dues. Returns the enrollment and how many dues were added.
func (s *Store) CreateEnrollment(studentID, batchID int, joinedMonth string) (Enrollment, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.studentByID(studentID); !ok {
		return Enrollment{}, 0, fmt.Errorf("student %d: %w", studentID, ErrNotFound)
	}
	if _, ok := s.batchByID(batchID); !ok {
		return Enrollment{}, 0, fmt.Errorf("batch %d: %w", batchID, ErrNotFound)
	}
	if _, _, ok := parseMonth(joinedMonth); !ok {
		return Enrollment{}, 0, fmt.Errorf("joinedMonth must be YYYY-MM")
	}
	if joinedMonth > AnchorMonth {
		return Enrollment{}, 0, fmt.Errorf("joinedMonth cannot be after %s", AnchorMonth)
	}
	for _, e := range s.d.Enrollments {
		if e.StudentID == studentID && e.BatchID == batchID {
			return Enrollment{}, 0, fmt.Errorf("student is already enrolled in this batch")
		}
	}
	e := Enrollment{ID: s.nextID(), StudentID: studentID, BatchID: batchID, JoinedMonth: joinedMonth}
	s.d.Enrollments = append(s.d.Enrollments, e)
	added := s.ensureDuesLocked(AnchorMonth)
	s.save()
	return e, added, nil
}

func (s *Store) DeleteEnrollment(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.enrollmentByID(id); !ok {
		return fmt.Errorf("enrollment %d: %w", id, ErrNotFound)
	}
	s.d.Enrollments = filter(s.d.Enrollments, func(e Enrollment) bool { return e.ID != id })
	s.d.Dues = filter(s.d.Dues, func(d Due) bool { return d.EnrollmentID != id })
	s.d.Payments = filter(s.d.Payments, func(p Payment) bool { return p.EnrollmentID != id })
	s.save()
	return nil
}

// ---------- dues & payments ----------

// ensureDuesLocked generates every missing due through currentMonth.
// Idempotent; callers must hold s.mu.
func (s *Store) ensureDuesLocked(currentMonth string) int {
	missing := MissingDues(s.d.Enrollments, s.feeOfLocked, s.d.Dues, currentMonth)
	for i := range missing {
		missing[i].ID = s.nextID()
		s.d.Dues = append(s.d.Dues, missing[i])
	}
	return len(missing)
}

// EnsureDues regenerates dues through the anchor month; safe to call any
// number of times.
func (s *Store) EnsureDues() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	added := s.ensureDuesLocked(AnchorMonth)
	if added > 0 {
		s.save()
	}
	return added
}

// AddPayment records a (possibly partial) payment against one
// enrollment-month.
func (s *Store) AddPayment(p Payment) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.enrollmentByID(p.EnrollmentID)
	if !ok {
		return Payment{}, fmt.Errorf("enrollment %d: %w", p.EnrollmentID, ErrNotFound)
	}
	if p.Amount <= 0 {
		return Payment{}, fmt.Errorf("payment amount must be greater than 0")
	}
	p.Mode = strings.ToLower(strings.TrimSpace(p.Mode))
	if p.Mode != "cash" && p.Mode != "upi" {
		return Payment{}, fmt.Errorf("mode must be cash or upi")
	}
	if _, _, ok := parseMonth(p.Month); !ok {
		return Payment{}, fmt.Errorf("month must be YYYY-MM")
	}
	if p.Month < e.JoinedMonth || p.Month > AnchorMonth {
		return Payment{}, fmt.Errorf("month must be between %s and %s for this enrollment", e.JoinedMonth, AnchorMonth)
	}
	if p.Date == "" {
		p.Date = AnchorDate
	}
	if _, err := time.Parse("2006-01-02", p.Date); err != nil {
		return Payment{}, fmt.Errorf("date must be YYYY-MM-DD")
	}
	s.ensureDuesLocked(AnchorMonth)
	p.ID = s.nextID()
	s.d.Payments = append(s.d.Payments, p)
	s.save()
	return p, nil
}

// ---------- attendance ----------

// MarkAttendance saves a batch-day register idempotently: re-saving the same
// grid changes nothing and sends no duplicate alerts. A student newly marked
// absent triggers one absence alert into the outbox.
func (s *Store) MarkAttendance(batchID int, date string, marks map[int]bool, p Provider, now time.Time) (AttendanceRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, alerts, err := s.markAttendanceLocked(batchID, date, marks, p, now)
	if err == nil {
		s.save()
	}
	return rec, alerts, err
}

func (s *Store) markAttendanceLocked(batchID int, date string, marks map[int]bool, p Provider, now time.Time) (AttendanceRecord, int, error) {
	b, ok := s.batchByID(batchID)
	if !ok {
		return AttendanceRecord{}, 0, fmt.Errorf("batch %d: %w", batchID, ErrNotFound)
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return AttendanceRecord{}, 0, fmt.Errorf("date must be YYYY-MM-DD")
	}
	enrolled := map[int]bool{}
	for _, e := range s.d.Enrollments {
		if e.BatchID == batchID {
			enrolled[e.StudentID] = true
		}
	}
	ids := make([]int, 0, len(marks))
	for sid := range marks {
		if !enrolled[sid] {
			return AttendanceRecord{}, 0, fmt.Errorf("student %d is not enrolled in %s", sid, b.Name)
		}
		ids = append(ids, sid)
	}
	sort.Ints(ids)

	var rec *AttendanceRecord
	for i := range s.d.Attendance {
		r := &s.d.Attendance[i]
		if r.BatchID == batchID && r.Date == date {
			rec = r
			break
		}
	}
	if rec == nil {
		s.d.Attendance = append(s.d.Attendance, AttendanceRecord{BatchID: batchID, Date: date, Marks: map[int]bool{}})
		rec = &s.d.Attendance[len(s.d.Attendance)-1]
	}

	alerts := 0
	for _, sid := range ids {
		present := marks[sid]
		prev, had := rec.Marks[sid]
		rec.Marks[sid] = present
		if !present && (!had || prev) {
			st, ok := s.studentByID(sid)
			if !ok {
				continue
			}
			body := AbsenceMessage(st.ParentName, st.Name, b.Name, date)
			s.addOutboxLocked(p, "absence", st, b.Name, body, now)
			alerts++
		}
	}

	out := AttendanceRecord{BatchID: rec.BatchID, Date: rec.Date, Marks: make(map[int]bool, len(rec.Marks))}
	for k, v := range rec.Marks {
		out.Marks[k] = v
	}
	return out, alerts, nil
}

// ---------- messaging ----------

func (s *Store) addOutboxLocked(p Provider, typ string, st Student, batchName, body string, now time.Time) {
	msgID, err := p.Send(st.ParentPhone, body)
	if err != nil {
		msgID = "send-failed"
	}
	s.d.Outbox = append(s.d.Outbox, OutboxMessage{
		ID:            s.nextID(),
		Type:          typ,
		To:            st.ParentPhone,
		ParentName:    st.ParentName,
		StudentName:   st.Name,
		BatchName:     batchName,
		Body:          body,
		ProviderMsgID: msgID,
		CreatedAt:     now.In(IST).Format(time.RFC3339),
		Status:        "queued",
	})
}

// SendFeeReminders queues one bilingual reminder per enrollment that still
// owes money, listing the pending months and the total outstanding.
func (s *Store) SendFeeReminders(p Provider, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDuesLocked(AnchorMonth)
	sent := 0
	for _, e := range s.d.Enrollments {
		var months []string
		var total int64
		for _, d := range s.d.Dues {
			if d.EnrollmentID != e.ID {
				continue
			}
			if out := OutstandingForDue(d, s.d.Payments); out > 0 {
				months = append(months, MonthLabel(d.Month))
				total += out
			}
		}
		if total == 0 {
			continue
		}
		st, ok1 := s.studentByID(e.StudentID)
		b, ok2 := s.batchByID(e.BatchID)
		if !ok1 || !ok2 {
			continue
		}
		body := FeeReminderMessage(st.ParentName, st.Name, b.Name, months, total)
		s.addOutboxLocked(p, "fee_reminder", st, b.Name, body, now)
		sent++
	}
	s.save()
	return sent
}

// Announce queues one personalized message per parent enrolled in the batch.
func (s *Store) Announce(batchID int, message string, p Provider, now time.Time) (int, error) {
	if strings.TrimSpace(message) == "" {
		return 0, fmt.Errorf("announcement message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batchByID(batchID)
	if !ok {
		return 0, fmt.Errorf("batch %d: %w", batchID, ErrNotFound)
	}
	sent := 0
	for _, e := range s.d.Enrollments {
		if e.BatchID != batchID {
			continue
		}
		st, ok := s.studentByID(e.StudentID)
		if !ok {
			continue
		}
		body := AnnouncementMessage(st.ParentName, st.Name, b.Name, message)
		s.addOutboxLocked(p, "announcement", st, b.Name, body, now)
		sent++
	}
	s.save()
	return sent, nil
}

// ---------- helpers ----------

func filter[T any](in []T, keep func(T) bool) []T {
	out := in[:0]
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	// Re-slice into a fresh backing array to avoid aliasing surprises.
	return append([]T(nil), out...)
}
