package utils

import "time"

// Time duration constants to avoid magic numbers
const (
	// JWT token expiration times
	JWTTokenExpiration      = 24 * time.Hour      // 1 day
	JWTRefreshTokenExpiration = 7 * 24 * time.Hour // 7 days

	// Common time durations
	OneDay   = 24 * time.Hour
	OneWeek  = 7 * 24 * time.Hour
	OneMonth = 30 * 24 * time.Hour
	OneYear  = 365 * 24 * time.Hour
)
