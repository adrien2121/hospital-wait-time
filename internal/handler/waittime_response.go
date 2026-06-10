package handler

import (
	"time"

	"github.com/adrien2121/GoProject/internal/domain"
)

// snapshotResponse is one wait time reading.
// Used inside statusResponse and as array elements in GET .../wait-times/history.
type snapshotResponse struct {
	ID          int64  `json:"id"`
	HospitalID  string `json:"hospital_id"`
	WaitMinutes int    `json:"wait_minutes"`
	Category    string `json:"category"`
	RecordedAt  string `json:"recorded_at"`
	ScrapedAt   string `json:"scraped_at"`
}

// signalResponse is one external signal reading attached to a status response.
type signalResponse struct {
	SignalName string  `json:"signal_name"`
	Value      float64 `json:"value"`
	ObservedAt string  `json:"observed_at"`
}

// statusResponse is the current status for one hospital.
// Used in GET /api/v1/wait-times/current, /summary, and /hospitals/{id}/wait-times/current.
type statusResponse struct {
	Hospital       hospitalResponse  `json:"hospital"`
	Latest         *snapshotResponse `json:"latest,omitempty"`
	Trend          string            `json:"trend"`
	IsUnusual      bool              `json:"is_unusual"`
	ScrapedAgoSecs int               `json:"scraped_ago_seconds,omitempty"`
	Signals        []signalResponse  `json:"signals,omitempty"`
}

// bestTimeResponse is returned by GET /api/v1/hospitals/{id}/wait-times/best-time.
type bestTimeResponse struct {
	HospitalID     string  `json:"hospital_id"`
	DayOfWeek      string  `json:"day_of_week"`
	Hour           int     `json:"hour"`
	AvgWaitMinutes float64 `json:"avg_wait_minutes"`
}

// Maps WaitTimeSnapshot domain to JSON response shape.
func toSnapshotResponse(s domain.WaitTimeSnapshot) snapshotResponse {
	return snapshotResponse{
		ID:          s.ID,
		HospitalID:  s.HospitalID,
		WaitMinutes: s.WaitMinutes,
		Category:    string(s.Category),
		RecordedAt:  s.RecordedAt.Format(time.RFC3339),
		ScrapedAt:   s.ScrapedAt.Format(time.RFC3339),
	}
}

// Maps HospitalStatus domain to JSON response shape.
func toStatusResponse(s domain.HospitalStatus) statusResponse {
	resp := statusResponse{
		Hospital:  toHospitalResponse(s.Hospital),
		Trend:     string(s.Trend),
		IsUnusual: s.IsUnusual,
	}
	if s.Latest != nil {
		snap := toSnapshotResponse(*s.Latest)
		resp.Latest = &snap
		resp.ScrapedAgoSecs = int(s.ScrapedAgo.Seconds())
	}
	if len(s.Signals) > 0 {
		resp.Signals = make([]signalResponse, len(s.Signals))
		for i, sig := range s.Signals {
			resp.Signals[i] = signalResponse{
				SignalName: string(sig.SignalName),
				Value:      sig.Value,
				ObservedAt: sig.ObservedAt.Format(time.RFC3339),
			}
		}
	}
	return resp
}

// Maps a slice of HospitalStatus domain to JSON response shapes.
func toStatusResponses(statuses []domain.HospitalStatus) []statusResponse {
	resp := make([]statusResponse, len(statuses))
	for i, s := range statuses {
		resp[i] = toStatusResponse(s)
	}
	return resp
}
