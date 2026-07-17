package core

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// Provider is the WhatsApp integration point. The MVP ships only the
// deterministic mock; a live implementation would call the WhatsApp Business
// Cloud API using WHATSAPP_TOKEN / WHATSAPP_PHONE_NUMBER_ID.
type Provider interface {
	Send(toPhone, body string) (providerMsgID string, err error)
	Mode() string
}

// MockWhatsApp is the zero-key stand-in for the WhatsApp Business API.
// The same recipient + body always yields the same message id (FNV-1a).
type MockWhatsApp struct{}

// Send returns a deterministic wamid-style id and never fails.
func (MockWhatsApp) Send(to, body string) (string, error) {
	h := fnv.New32a()
	h.Write([]byte(to + "|" + body))
	return fmt.Sprintf("wamid.MOCK-%08X", h.Sum32()), nil
}

// Mode reports "mock" so /health can surface the provider state.
func (MockWhatsApp) Mode() string { return "mock" }

// AbsenceMessage is the bilingual alert sent when a student is marked absent.
func AbsenceMessage(parent, student, batch, date string) string {
	label := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		label = t.Format("Mon, 2 Jan 2006")
	}
	return fmt.Sprintf(
		"Dear %s, your ward %s was marked ABSENT in %s on %s. Please inform the institute if this was expected. — ClassKhata\n"+
			"प्रिय %s जी, आपके बच्चे %s आज %s की कक्षा (%s) में अनुपस्थित रहे। कृपया संस्थान को सूचित करें। — ClassKhata",
		parent, student, batch, label, parent, student, batch, label)
}

// FeeReminderMessage is the bilingual dues reminder for one enrollment.
func FeeReminderMessage(parent, student, batch string, monthLabels []string, outstanding int64) string {
	months := strings.Join(monthLabels, ", ")
	amount := FormatINR(outstanding)
	return fmt.Sprintf(
		"Dear %s, the fee for %s (%s) is pending: %s for %s. Kindly pay by cash or UPI at the institute. — ClassKhata\n"+
			"प्रिय %s जी, %s (%s) की %s फीस (%s) बकाया है। कृपया शीघ्र भुगतान करें। धन्यवाद! — ClassKhata",
		parent, student, batch, amount, months, parent, student, batch, months, amount)
}

// AnnouncementMessage personalizes a batch broadcast for one parent.
func AnnouncementMessage(parent, student, batch, message string) string {
	return fmt.Sprintf("Dear %s (parent of %s, %s): %s — ClassKhata", parent, student, batch, message)
}
