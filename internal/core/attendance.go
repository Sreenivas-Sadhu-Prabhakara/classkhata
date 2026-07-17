package core

import "math"

// AttendanceStats counts a student's marks across records. batchID 0 means
// "across all batches". Unmarked days do not count toward the total.
func AttendanceStats(records []AttendanceRecord, studentID, batchID int) (present, total int) {
	for _, r := range records {
		if batchID != 0 && r.BatchID != batchID {
			continue
		}
		p, marked := r.Marks[studentID]
		if !marked {
			continue
		}
		total++
		if p {
			present++
		}
	}
	return present, total
}

// AttendancePercent is the student's presence rate as a percentage rounded
// to one decimal. A student with no marks yet is 0.
func AttendancePercent(records []AttendanceRecord, studentID, batchID int) float64 {
	present, total := AttendanceStats(records, studentID, batchID)
	if total == 0 {
		return 0
	}
	return math.Round(float64(present)/float64(total)*1000) / 10
}
