package service

import "testing"

// dayOfWeekName maps a PostgreSQL EXTRACT(dow) integer (0=Sunday … 6=Saturday) to
// its English name for the public best-time API response.
func TestDayOfWeekName(t *testing.T) {
	tests := []struct {
		name string
		dow  int
		want string
	}{
		{
			name: `
				given a 0 from PostgreSQL EXTRACT(dow),
				when dayOfWeekName maps it,
				then the public API sees 'Sunday'`,
			dow:  0,
			want: "Sunday",
		},
		{
			name: `
				given a 6 from PostgreSQL EXTRACT(dow),
				when dayOfWeekName maps it,
				then the public API sees 'Saturday'`,
			dow:  6,
			want: "Saturday",
		},
		{
			name: `
				given a 3 from PostgreSQL EXTRACT(dow),
				when dayOfWeekName maps it,
				then the public API sees 'Wednesday'`,
			dow:  3,
			want: "Wednesday",
		},
		{
			name: `
				given a corrupt negative value
				(should never happen, but a SQL change could produce one),
				when dayOfWeekName is called,
				then it returns 'Unknown' instead of indexing out of range`,
			dow:  -1,
			want: "Unknown",
		},
		{
			name: `
				given a value above 6 (same corruption concern),
				when dayOfWeekName is called,
				then it returns 'Unknown'`,
			dow:  7,
			want: "Unknown",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the EXTRACT(dow) integer from the table row.
			dow := tc.dow

			// When: the best-time service asks dayOfWeekName to convert it.
			got := dayOfWeekName(dow)

			// Then: the API sees the expected English weekday name.
			if got != tc.want {
				t.Fatalf("dayOfWeekName(%d) = %q, want %q", dow, got, tc.want)
			}
		})
	}
}
