// Package api wires ClassKhata's JSON API and the embedded web UI onto one
// http.Handler using Go 1.22 pattern routing.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"classkhata/internal/core"
)

type server struct {
	store    *core.Store
	provider core.Provider
}

// New builds the full handler: /api/v1/* plus the embedded static UI.
func New(store *core.Store, provider core.Provider, static fs.FS) http.Handler {
	s := &server{store: store, provider: provider}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.health)

	mux.HandleFunc("GET /api/v1/batches", s.listBatches)
	mux.HandleFunc("POST /api/v1/batches", s.createBatch)
	mux.HandleFunc("GET /api/v1/batches/{id}", s.getBatch)
	mux.HandleFunc("PUT /api/v1/batches/{id}", s.updateBatch)
	mux.HandleFunc("DELETE /api/v1/batches/{id}", s.deleteBatch)

	mux.HandleFunc("GET /api/v1/students", s.listStudents)
	mux.HandleFunc("POST /api/v1/students", s.createStudent)
	mux.HandleFunc("GET /api/v1/students/{id}", s.getStudent)
	mux.HandleFunc("PUT /api/v1/students/{id}", s.updateStudent)
	mux.HandleFunc("DELETE /api/v1/students/{id}", s.deleteStudent)

	mux.HandleFunc("GET /api/v1/enrollments", s.listEnrollments)
	mux.HandleFunc("POST /api/v1/enrollments", s.createEnrollment)
	mux.HandleFunc("DELETE /api/v1/enrollments/{id}", s.deleteEnrollment)

	mux.HandleFunc("GET /api/v1/attendance", s.getAttendance)
	mux.HandleFunc("POST /api/v1/attendance", s.postAttendance)

	mux.HandleFunc("GET /api/v1/dues", s.listDues)
	mux.HandleFunc("POST /api/v1/dues/generate", s.generateDues)

	mux.HandleFunc("GET /api/v1/payments", s.listPayments)
	mux.HandleFunc("POST /api/v1/payments", s.createPayment)

	mux.HandleFunc("POST /api/v1/reminders/fees", s.sendFeeReminders)
	mux.HandleFunc("POST /api/v1/announcements", s.announce)
	mux.HandleFunc("GET /api/v1/outbox", s.listOutbox)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboard)
	mux.HandleFunc("POST /api/v1/demo", s.seedDemo)

	mux.Handle("GET /", http.FileServerFS(static))
	return mux
}

// ---------- plumbing ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}

func fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, core.ErrNotFound) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

// ---------- handlers ----------

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":           "ok",
		"app":              "classkhata",
		"whatsappProvider": s.provider.Mode(),
		"anchorDate":       core.AnchorDate,
	})
}

func (s *server) listBatches(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.BatchViews())
}

func (s *server) createBatch(w http.ResponseWriter, r *http.Request) {
	var b core.Batch
	if !readJSON(w, r, &b) {
		return
	}
	created, err := s.store.CreateBatch(b)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	v, ok := s.store.BatchView(id)
	if !ok {
		fail(w, core.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) updateBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var b core.Batch
	if !readJSON(w, r, &b) {
		return
	}
	updated, err := s.store.UpdateBatch(id, b)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.DeleteBatch(id); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *server) listStudents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.StudentViews())
}

func (s *server) createStudent(w http.ResponseWriter, r *http.Request) {
	var st core.Student
	if !readJSON(w, r, &st) {
		return
	}
	created, err := s.store.CreateStudent(st)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getStudent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	v, ok := s.store.StudentView(id)
	if !ok {
		fail(w, core.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) updateStudent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var st core.Student
	if !readJSON(w, r, &st) {
		return
	}
	updated, err := s.store.UpdateStudent(id, st)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteStudent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.DeleteStudent(id); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *server) listEnrollments(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Enrollments())
}

func (s *server) createEnrollment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StudentID   int    `json:"studentId"`
		BatchID     int    `json:"batchId"`
		JoinedMonth string `json:"joinedMonth"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	e, dues, err := s.store.CreateEnrollment(req.StudentID, req.BatchID, req.JoinedMonth)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"enrollment": e, "duesGenerated": dues})
}

func (s *server) deleteEnrollment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.DeleteEnrollment(id); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *server) getAttendance(w http.ResponseWriter, r *http.Request) {
	batchID, err := strconv.Atoi(r.URL.Query().Get("batchId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "batchId query parameter is required"})
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = core.AnchorDate
	}
	grid, err := s.store.AttendanceGrid(batchID, date)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grid)
}

func (s *server) postAttendance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BatchID int          `json:"batchId"`
		Date    string       `json:"date"`
		Marks   map[int]bool `json:"marks"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Marks) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "marks must contain at least one student"})
		return
	}
	rec, alerts, err := s.store.MarkAttendance(req.BatchID, req.Date, req.Marks, s.provider, time.Now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "alertsSent": alerts, "record": rec})
}

func (s *server) listDues(w http.ResponseWriter, _ *http.Request) {
	rows := s.store.DueRows()
	var total int64
	for _, r := range rows {
		total += r.Outstanding
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows":                      rows,
		"totalOutstanding":          total,
		"totalOutstandingFormatted": core.FormatINR(total),
	})
}

func (s *server) generateDues(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int{"generated": s.store.EnsureDues()})
}

func (s *server) listPayments(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.PaymentViews())
}

func (s *server) createPayment(w http.ResponseWriter, r *http.Request) {
	var p core.Payment
	if !readJSON(w, r, &p) {
		return
	}
	created, err := s.store.AddPayment(p)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) sendFeeReminders(w http.ResponseWriter, _ *http.Request) {
	sent := s.store.SendFeeReminders(s.provider, time.Now())
	writeJSON(w, http.StatusOK, map[string]int{"sent": sent})
}

func (s *server) announce(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BatchID int    `json:"batchId"`
		Message string `json:"message"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	sent, err := s.store.Announce(req.BatchID, req.Message, s.provider, time.Now())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"sent": sent})
}

func (s *server) listOutbox(w http.ResponseWriter, _ *http.Request) {
	msgs := s.store.OutboxList()
	writeJSON(w, http.StatusOK, map[string]any{"count": len(msgs), "messages": msgs})
}

func (s *server) dashboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.DashboardView())
}

func (s *server) seedDemo(w http.ResponseWriter, _ *http.Request) {
	counts := s.store.SeedDemo(s.provider)
	writeJSON(w, http.StatusOK, map[string]any{"seeded": true, "counts": counts})
}
