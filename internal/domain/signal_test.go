package domain

import "testing"

// IsRegional is the call-site replacement for `signal.HospitalID == nil` checks.
// These tests pin the contract: nil pointer means regional (weather, AQHI), a
// non-nil pointer means hospital-specific.
func TestExternalSignal_IsRegional(t *testing.T) {
	hospitalID := "toh-civic"
	tests := []struct {
		name string
		sig  ExternalSignal
		want bool
	}{
		{
			name: `
				given a signal stored with no hospital
				(a weather or AQHI reading that applies to every facility),
				when IsRegional is called,
				then it returns true`,
			sig:  ExternalSignal{HospitalID: nil},
			want: true,
		},
		{
			name: `
				given a signal tied to a specific hospital
				(e.g. a sibling-facility queue reading),
				when IsRegional is called,
				then it returns false`,
			sig:  ExternalSignal{HospitalID: &hospitalID},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the signal from the table row.
			sig := tc.sig

			// When: a caller asks whether this signal applies to every hospital.
			got := sig.IsRegional()

			// Then: the answer matches the contract for nil vs non-nil HospitalID.
			if got != tc.want {
				t.Fatalf("IsRegional() = %v, want %v", got, tc.want)
			}
		})
	}
}
