package billing

import "time"

// ExtendExpiry starts the new period at the later of the current expiry and the
// approval instant. durationMonths are calendar months (time.AddDate), not a fixed
// number of days. A date that does not exist in the target month is normalized
// by the Go time package.
func ExtendExpiry(current *time.Time, approved time.Time, durationMonths int) (start, end time.Time) {
	approved = approved.UTC()
	start = approved
	if current != nil && current.UTC().After(start) {
		start = current.UTC()
	}
	if durationMonths < 1 {
		durationMonths = 1
	}
	return start, start.AddDate(0, durationMonths, 0)
}
