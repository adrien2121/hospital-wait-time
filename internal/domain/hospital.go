package domain

// FacilityType classifies a healthcare facility.
type FacilityType string

const (
	FacilityTypeER     FacilityType = "er"
	FacilityTypeClinic FacilityType = "clinic"
)

// Stable hospital identifiers shared by scrapers, services, and the hospitals table.
// Every value must match a row in the hospitals table.
const (
	HospitalIDTOHCivic   = "toh-civic"
	HospitalIDTOHGeneral = "toh-general"
	HospitalIDCHEO       = "cheo"
	HospitalIDQCH        = "qch"
	HospitalIDMontfort   = "montfort"
)

// Hospital represents an Ottawa-area healthcare facility.
type Hospital struct {
	ID           string
	Name         string
	Address      string
	FacilityType FacilityType
	SourceURL    string
	Active       bool
}
