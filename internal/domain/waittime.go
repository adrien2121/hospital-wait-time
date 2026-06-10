package domain

import "time"

// WaitCategory identifies what a wait time measurement represents.
type WaitCategory string

const (
	WaitCategoryTriageToDoctor WaitCategory = "triage_to_doctor"
	WaitCategoryTotalStayLow   WaitCategory = "total_stay_low_urgency"
	WaitCategoryTotalStayHigh  WaitCategory = "total_stay_high_urgency"
)

// TrendDirection indicates whether wait times are improving or worsening.
type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
)

// WaitTimeSnapshot is a single point-in-time wait time reading for a facility.
type WaitTimeSnapshot struct {
	ID          int64
	HospitalID  string
	WaitMinutes int
	Category    WaitCategory
	RecordedAt  time.Time // time the hospital published this value
	ScrapedAt   time.Time // time we captured it
}

// HospitalStatus combines a hospital with its latest snapshot and derived metadata.
// Latest is a pointer because a hospital may exist before any snapshot is scraped.
type HospitalStatus struct {
	Hospital   Hospital
	Latest     *WaitTimeSnapshot
	Trend      TrendDirection
	IsUnusual  bool
	ScrapedAgo time.Duration
	Signals    []ExternalSignal // latest value per signal name at time of request
}

// BestTimeSlot is the historically lowest-wait time window for a facility.
type BestTimeSlot struct {
	HospitalID     string
	DayOfWeek      string
	Hour           int
	AvgWaitMinutes float64
}
