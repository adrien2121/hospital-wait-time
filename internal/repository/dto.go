package repository

// HourlyAverage is a GROUP BY hour/day-of-week aggregation result.
// Query-result DTO tied to the DB aggregation shape, not a domain entity.
type HourlyAverage struct {
	HospitalID     string
	DayOfWeek      int // 0 = Sunday, 6 = Saturday (PostgreSQL EXTRACT(dow))
	Hour           int
	AvgWaitMinutes float64
}
