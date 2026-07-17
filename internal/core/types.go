// Package core holds ClassKhata's domain: batches, students, enrollments,
// the fees engine, attendance math and the WhatsApp outbox. Everything here
// is deterministic and stdlib-only.
package core

// AnchorDate is the fixed "today" ClassKhata reasons about. Keeping it fixed
// makes demo data, dues generation and dashboard numbers fully deterministic.
const AnchorDate = "2026-07-17"

// AnchorMonth is the current billing month derived from AnchorDate.
const AnchorMonth = "2026-07"

// ValidDays are the schedule-day tokens a batch may use.
var ValidDays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// Batch is a class group with a weekly schedule and a monthly fee in rupees.
type Batch struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Subject    string   `json:"subject"`
	Days       []string `json:"days"`
	StartTime  string   `json:"startTime"` // "18:00" (24h)
	EndTime    string   `json:"endTime"`   // "19:30"
	MonthlyFee int64    `json:"monthlyFee"`
}

// Student is a learner plus the parent contact ClassKhata messages.
type Student struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ParentName  string `json:"parentName"`
	ParentPhone string `json:"parentPhone"` // normalized "+91-XXXXXXXXXX"
}

// Enrollment links a student to a batch from a joining month onwards.
type Enrollment struct {
	ID          int    `json:"id"`
	StudentID   int    `json:"studentId"`
	BatchID     int    `json:"batchId"`
	JoinedMonth string `json:"joinedMonth"` // "2026-04"
}

// Due is one month's fee owed by one enrollment.
type Due struct {
	ID           int    `json:"id"`
	EnrollmentID int    `json:"enrollmentId"`
	Month        string `json:"month"`
	Amount       int64  `json:"amount"`
}

// Payment is money received against an enrollment for a given month.
// Partial payments are allowed; several payments may target one due.
type Payment struct {
	ID           int    `json:"id"`
	EnrollmentID int    `json:"enrollmentId"`
	Month        string `json:"month"`
	Amount       int64  `json:"amount"`
	Mode         string `json:"mode"` // "cash" | "upi"
	Date         string `json:"date"` // "2026-07-05"
}

// AttendanceRecord stores one batch-day register: studentID -> present.
type AttendanceRecord struct {
	BatchID int          `json:"batchId"`
	Date    string       `json:"date"`
	Marks   map[int]bool `json:"marks"`
}

// OutboxMessage is a WhatsApp message queued through the provider (mock by
// default). ProviderMsgID is deterministic for the mock provider.
type OutboxMessage struct {
	ID            int    `json:"id"`
	Type          string `json:"type"` // "absence" | "fee_reminder" | "announcement"
	To            string `json:"to"`
	ParentName    string `json:"parentName"`
	StudentName   string `json:"studentName"`
	BatchName     string `json:"batchName"`
	Body          string `json:"body"`
	ProviderMsgID string `json:"providerMsgId"`
	CreatedAt     string `json:"createdAt"`
	Status        string `json:"status"`
}
