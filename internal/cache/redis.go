package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/go-redis/redis/v8"
)

type RedisCache struct {
    client *redis.Client
    ctx    context.Context
}

func NewRedisCache() (*RedisCache, error) {
    host := os.Getenv("REDIS_HOST")
    if host == "" {
        host = "localhost"
    }

    port := os.Getenv("REDIS_PORT")
    if port == "" {
        port = "6379"
    }

    addr := fmt.Sprintf("%s:%s", host, port)

    client := redis.NewClient(&redis.Options{
        Addr: addr,
        DB:   0,
    })

    ctx := context.Background()

    if err := client.Ping(ctx).Err(); err != nil {
        log.Printf("⚠️ Redis connection failed: %v, will use fallback only", err)
        return &RedisCache{
            client: client,
            ctx:    ctx,
        }, nil
    }

    log.Printf("✅ Connected to Redis at %s", addr)
    return &RedisCache{
        client: client,
        ctx:    ctx,
    }, nil
}

func (r *RedisCache) Set(key string, value interface{}, ttl int) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return r.client.Set(r.ctx, key, data, time.Duration(ttl)*time.Millisecond).Err()
}

func (r *RedisCache) Get(key string, dest interface{}) error {
    data, err := r.client.Get(r.ctx, key).Bytes()
    if err != nil {
        return err
    }
    return json.Unmarshal(data, dest)
}

func (r *RedisCache) Ping() error {
    return r.client.Ping(r.ctx).Err()
}
