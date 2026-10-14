package authn

import "time"

// Config holds the authentication limits. Zero durations are replaced by DefaultConfig.
type Config struct {
	JWTSecret      string
	OTPSecret      string
	Issuer         string
	Audience       string
	Region         string
	AccessTTL      time.Duration
	IdleTTL        time.Duration
	AbsoluteTTL    time.Duration
	OTPTTL         time.Duration
	OTPMaxAttempts int
	OTPResend      time.Duration
	GrantTTL       time.Duration
	BcryptCost     int
	OTPPerPhone    int
	OTPPerIP       int
	OTPWindow      time.Duration
	LoginAttempts  int
	LoginWindow    time.Duration
	// SeededOTP maps an E.164 number to a fixed code used to test OTP.
	SeededOTP map[string]string
	Now       func() time.Time
}

func DefaultConfig() Config {
	return Config{
		Issuer:         "easystock",
		Audience:       "easystock",
		Region:         "CD",
		AccessTTL:      15 * time.Minute,
		IdleTTL:        30 * 24 * time.Hour,
		AbsoluteTTL:    90 * 24 * time.Hour,
		OTPTTL:         5 * time.Minute,
		OTPMaxAttempts: 5,
		OTPResend:      60 * time.Second,
		GrantTTL:       10 * time.Minute,
		OTPPerPhone:    5,
		OTPPerIP:       20,
		OTPWindow:      time.Hour,
		LoginAttempts:  10,
		LoginWindow:    15 * time.Minute,
	}
}

func (c Config) withDefaults() Config {
	base := DefaultConfig()
	if c.Issuer != "" {
		base.Issuer = c.Issuer
	}
	if c.Audience != "" {
		base.Audience = c.Audience
	}
	if c.Region != "" {
		base.Region = c.Region
	}
	if c.AccessTTL > 0 {
		base.AccessTTL = c.AccessTTL
	}
	if c.IdleTTL > 0 {
		base.IdleTTL = c.IdleTTL
	}
	if c.AbsoluteTTL > 0 {
		base.AbsoluteTTL = c.AbsoluteTTL
	}
	if c.OTPTTL > 0 {
		base.OTPTTL = c.OTPTTL
	}
	if c.OTPMaxAttempts > 0 {
		base.OTPMaxAttempts = c.OTPMaxAttempts
	}
	if c.OTPResend > 0 {
		base.OTPResend = c.OTPResend
	}
	if c.GrantTTL > 0 {
		base.GrantTTL = c.GrantTTL
	}
	if c.BcryptCost > 0 {
		base.BcryptCost = c.BcryptCost
	}
	if c.OTPPerPhone > 0 {
		base.OTPPerPhone = c.OTPPerPhone
	}
	if c.OTPPerIP > 0 {
		base.OTPPerIP = c.OTPPerIP
	}
	if c.OTPWindow > 0 {
		base.OTPWindow = c.OTPWindow
	}
	if c.LoginAttempts > 0 {
		base.LoginAttempts = c.LoginAttempts
	}
	if c.LoginWindow > 0 {
		base.LoginWindow = c.LoginWindow
	}
	base.JWTSecret = c.JWTSecret
	base.OTPSecret = c.OTPSecret
	base.SeededOTP = c.SeededOTP
	base.Now = c.Now
	return base
}

// DevelopmentOTPSeed is the phone and code used to exercise OTP without reading the SMS.
// The API loads it in every environment, including the cloud deployment.
func DevelopmentOTPSeed() map[string]string {
	return map[string]string{
		"+243978154329": "123123",
	}
}

func (c Config) seededCode(number string) (string, bool) {
	code, ok := c.SeededOTP[number]
	if !ok || len(code) != 6 {
		return "", false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return code, true
}
