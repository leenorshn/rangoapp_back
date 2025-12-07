package database

// Cache functionality is disabled - Redis not configured
// This file is kept for future use when Redis is available
// To enable cache:
// 1. Install Redis and add go-redis dependency: go get github.com/redis/go-redis/v9
// 2. Uncomment the code below and configure cache in connect.go
// 3. Update resolvers to use cache

/*
import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rangoapp/utils"
)

// CacheInterface defines the interface for caching operations
type CacheInterface interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// MemoryCache is a simple in-memory cache implementation
type MemoryCache struct {
	data map[string]cacheEntry
}

type cacheEntry struct {
	value      []byte
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]cacheEntry),
	}
}

// Get retrieves a value from cache
func (c *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	entry, exists := c.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	// Check if expired
	if time.Now().After(entry.expiration) {
		delete(c.data, key)
		return nil, fmt.Errorf("key expired")
	}

	return entry.value, nil
}

// Set stores a value in cache
func (c *MemoryCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	c.data[key] = cacheEntry{
		value:      value,
		expiration: time.Now().Add(expiration),
	}
	return nil
}

// Delete removes a key from cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	delete(c.data, key)
	return nil
}

// Exists checks if a key exists in cache
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	entry, exists := c.data[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	if time.Now().After(entry.expiration) {
		delete(c.data, key)
		return false, nil
	}

	return true, nil
}

// CacheHelper provides helper methods for caching sales data
type CacheHelper struct {
	cache CacheInterface
}

// NewCacheHelper creates a new cache helper
func NewCacheHelper(cache CacheInterface) *CacheHelper {
	return &CacheHelper{cache: cache}
}

// GetCachedSales retrieves cached sales data
func (ch *CacheHelper) GetCachedSales(ctx context.Context, key string) ([]*Sale, error) {
	if ch.cache == nil {
		return nil, fmt.Errorf("cache not configured")
	}

	data, err := ch.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var sales []*Sale
	if err := json.Unmarshal(data, &sales); err != nil {
		return nil, err
	}

	return sales, nil
}

// SetCachedSales stores sales data in cache
func (ch *CacheHelper) SetCachedSales(ctx context.Context, key string, sales []*Sale, expiration time.Duration) error {
	if ch.cache == nil {
		return nil // Cache not configured, silently skip
	}

	data, err := json.Marshal(sales)
	if err != nil {
		return err
	}

	return ch.cache.Set(ctx, key, data, expiration)
}

// GetCachedStats retrieves cached statistics
func (ch *CacheHelper) GetCachedStats(ctx context.Context, key string) (*SaleStats, error) {
	if ch.cache == nil {
		return nil, fmt.Errorf("cache not configured")
	}

	data, err := ch.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var stats SaleStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// SetCachedStats stores statistics in cache
func (ch *CacheHelper) SetCachedStats(ctx context.Context, key string, stats *SaleStats, expiration time.Duration) error {
	if ch.cache == nil {
		return nil // Cache not configured, silently skip
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return ch.cache.Set(ctx, key, data, expiration)
}

// GenerateCacheKey generates a cache key for sales queries
func GenerateCacheKey(storeIDs []string, period *string, currency *string, limit *int, offset *int) string {
	key := fmt.Sprintf("sales:")
	
	// Add store IDs
	for i, id := range storeIDs {
		if i > 0 {
			key += ","
		}
		key += id
	}
	
	// Add period
	if period != nil {
		key += ":period:" + *period
	}
	
	// Add currency
	if currency != nil {
		key += ":currency:" + *currency
	}
	
	// Add pagination
	if limit != nil {
		key += fmt.Sprintf(":limit:%d", *limit)
	}
	if offset != nil {
		key += fmt.Sprintf(":offset:%d", *offset)
	}
	
	return key
}

// GenerateStatsCacheKey generates a cache key for statistics
func GenerateStatsCacheKey(storeIDs []string, period *string, currency *string) string {
	key := fmt.Sprintf("sales:stats:")
	
	// Add store IDs
	for i, id := range storeIDs {
		if i > 0 {
			key += ","
		}
		key += id
	}
	
	// Add period
	if period != nil {
		key += ":period:" + *period
	}
	
	// Add currency
	if currency != nil {
		key += ":currency:" + *currency
	}
	
	return key
}

// InvalidateSalesCache invalidates all sales-related cache entries
func (ch *CacheHelper) InvalidateSalesCache(ctx context.Context, storeID string) error {
	if ch.cache == nil {
		return nil
	}

	// In a real Redis implementation, you would use pattern matching
	// For now, we'll just log that cache should be invalidated
	utils.Info(fmt.Sprintf("Cache invalidation requested for store: %s", storeID))
	
	// In a full Redis implementation, you would do:
	// keys, err := redisClient.Keys(ctx, fmt.Sprintf("sales:*%s*", storeID)).Result()
	// for _, key := range keys {
	//     redisClient.Del(ctx, key)
	// }
	
	return nil
}
*/
