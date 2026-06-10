package domain

import "time"

// SignalName is a dotted-namespace identifier for an external signal source.
// Known names are declared as constants below; any string value is valid.
type SignalName string

const (
	SignalWeatherTempC    SignalName = "weather.temp_c"
	SignalWeatherPrecipMM SignalName = "weather.precip_mm"
	SignalWeatherSnowCM   SignalName = "weather.snow_cm"

	SignalAQHI SignalName = "aqhi.index"
)

// ExternalSignal is a single observation from a non-hospital data source used
// to inform wait time estimates (weather, sibling hospital waits, events, etc.).
//
// HospitalID is a pointer because regional signals (weather, AQHI) apply to every
// hospital and persist as SQL NULL. Pointer matches pgx's encoding of nullable
// columns. Use IsRegional() at call sites instead of repeating `== nil` checks.
type ExternalSignal struct {
	ID         int64
	SignalName SignalName
	// HospitalID nil for regional signals (weather, flu) that apply to all hospitals.
	// Reads: WHERE hospital_id = $1 OR hospital_id IS NULL.
	HospitalID *string
	Value      float64
	RawJSON    []byte // nil if source payload not stored; otherwise raw JSON for re-parsing
	ObservedAt time.Time
	ScrapedAt  time.Time
}

// IsRegional reports whether this signal applies to all hospitals (HospitalID is NULL in SQL).
func (s ExternalSignal) IsRegional() bool { return s.HospitalID == nil }
