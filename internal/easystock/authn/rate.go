package authn

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Limiter counts attempts inside a window. A Redis outage must not restore a session.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// PGLimiter stores counters in PostgreSQL so several API instances share them.
type PGLimiter struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPGLimiter(pool *pgxpool.Pool) *PGLimiter {
	return &PGLimiter{pool: pool, now: time.Now}
}

func (l *PGLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now()
	if l.now != nil {
		now = l.now()
	}
	start := now.Truncate(window)
	var hits int
	err := l.pool.QueryRow(ctx, `
		INSERT INTO rate_limit_buckets (bucket_key, window_start, hits)
		VALUES ($1, $2, 1)
		ON CONFLICT (bucket_key, window_start)
		DO UPDATE SET hits = rate_limit_buckets.hits + 1
		RETURNING hits
	`, key, start).Scan(&hits)
	if err != nil {
		return false, err
	}
	return hits <= limit, nil
}

// RedisLimiter counts attempts in Redis. Losing Redis only resets counters.
type RedisLimiter struct {
	client *redis.Client
	now    func() time.Time
}

func NewRedisLimiter(url string) (*RedisLimiter, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &RedisLimiter{client: redis.NewClient(opt), now: time.Now}, nil
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now()
	if l.now != nil {
		now = l.now()
	}
	bucket := key + ":" + now.Truncate(window).UTC().Format(time.RFC3339)
	hits, err := l.client.Incr(ctx, bucket).Result()
	if err != nil {
		return false, err
	}
	if hits == 1 {
		_ = l.client.Expire(ctx, bucket, window+time.Minute).Err()
	}
	return hits <= int64(limit), nil
}
