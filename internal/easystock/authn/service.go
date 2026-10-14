package authn

import (
	"context"
	"strings"
	"time"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/notify"
	"rangoapp/internal/easystock/phone"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	PurposeSignup = "signup"
	PurposeReset  = "password_reset"
	ScopeFull     = "full"
	ScopePassword = "password_change"
)

// Request carries the caller network data stored only as a hash.
type Request struct {
	IP        string
	UserAgent string
}

// Service is the internal phone authentication module.
type Service struct {
	pool    *pgxpool.Pool
	cfg     Config
	sms     notify.SMSProvider
	limiter Limiter
}

func New(pool *pgxpool.Pool, cfg Config, sms notify.SMSProvider, limiter Limiter) *Service {
	cfg = cfg.withDefaults()
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if limiter == nil {
		limiter = NewPGLimiter(pool)
	}
	return &Service{pool: pool, cfg: cfg, sms: sms, limiter: limiter}
}

func (s *Service) now() time.Time { return s.cfg.Now().UTC() }

func (s *Service) normalize(raw string) (string, error) {
	return phone.Normalize(raw, s.cfg.Region)
}

func clipAgent(agent string) string {
	agent = strings.TrimSpace(agent)
	if len(agent) > 200 {
		return agent[:200]
	}
	return agent
}

func (s *Service) allow(ctx context.Context, key string, limit int, window time.Duration) error {
	ok, err := s.limiter.Allow(ctx, key, limit, window)
	if err != nil {
		return domain.Internal("La demande n'a pas pu être limitée.")
	}
	if !ok {
		return domain.FailedPrecondition("rate_limited", "Trop de tentatives. Réessayez plus tard.")
	}
	return nil
}
