package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"wallpaper-generator/internal/models"
)

// RedisClient wraps the official go-redis Client
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates and validates a connection to Redis
func NewRedisClient(addr string) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisClient{client: rdb}, nil
}

// Close gracefully terminates the connection pool
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetActiveWallpaper checks if a wallpaper is currently cached
func (r *RedisClient) GetActiveWallpaper(ctx context.Context, key string) (*models.Wallpaper, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		//Cache miss: key does not exist or the key is being used for the first time or the wallpaper is new and is not in redis DB
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var wp models.Wallpaper
	// unmarshal will parse the val and fill into the wp
	if err := json.Unmarshal([]byte(val), &wp); err != nil {
		return nil, fmt.Errorf("failed to deserialize wallpaper from redis: %w", err)
	}

	return &wp, nil
}

// SetActiveWallpaper saves the active wallpaper with an expiration duration (TTL)
func (r *RedisClient) SetActiveWallpaper(ctx context.Context, key string, wp *models.Wallpaper, ttl time.Duration) error {
	data, err := json.Marshal(wp)
	if err != nil {
		return fmt.Errorf("failed to serialize wallpaper for redis: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}