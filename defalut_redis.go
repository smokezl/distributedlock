package distributedlock

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
)

var (
    // redis default
    defaultMaxRetry     = 2
    defaultMaxIdle      = 50
    defaultMaxActive    = 500
    defaultIdleTimeout  = 7200
    defaultConnTimeout  = 100
    defaultWriteTimeout = 250
    defaultReadTimeout  = 250
)

type RedisConfig struct {
    Addr         string `json:"addr"`
    Pwd          string `json:"pwd"`
    MaxRetry     int    `json:"max_retry"`
    MaxIdle      int    `json:"max_idle"`
    MaxActive    int    `json:"max_active"`
    IdleTimeout  int    `json:"idle_timeout"`  // 单位：s
    ConnTimeout  int    `json:"conn_timeout"`  // 单位：ms
    WriteTimeout int    `json:"write_timeout"` // 单位：ms
    ReadTimeout  int    `json:"read_timeout"`  // 单位：ms
}

func (c *RedisConfig) init() {
    if c.MaxRetry == 0 {
        c.MaxRetry = defaultMaxRetry
    }
    if c.MaxIdle == 0 {
        c.MaxIdle = defaultMaxIdle
    }
    if c.MaxActive == 0 {
        c.MaxActive = defaultMaxActive
    }
    if c.IdleTimeout == 0 {
        c.IdleTimeout = defaultIdleTimeout
    }
    if c.ConnTimeout == 0 {
        c.ConnTimeout = defaultConnTimeout
    }
    if c.WriteTimeout == 0 {
        c.WriteTimeout = defaultWriteTimeout
    }
    if c.ReadTimeout == 0 {
        c.ReadTimeout = defaultReadTimeout
    }
}

type defaultRedis struct {
    c      *RedisConfig
    client *redis.Client
}

func UseDefaultRedis(c *RedisConfig) DistributedI {
    c.init()
    return &defaultRedis{
        c: c,
        client: redis.NewClient(&redis.Options{
            Addr:            c.Addr,
            Password:        c.Pwd,
            MaxRetries:      c.MaxRetry,
            MaxIdleConns:    c.MaxIdle,
            MaxActiveConns:  c.MaxActive,
            ConnMaxLifetime: time.Duration(c.IdleTimeout) * time.Second,
            DialTimeout:     time.Duration(c.ConnTimeout) * time.Millisecond,
            WriteTimeout:    time.Duration(c.WriteTimeout) * time.Millisecond,
            ReadTimeout:     time.Duration(c.ReadTimeout) * time.Millisecond,
        }),
    }
}

func (d *defaultRedis) SetNX(ctx context.Context, key, value string, ttl int) (bool, error) {
    return d.client.SetNX(ctx, key, value, time.Duration(ttl)*time.Second).Result()
}

func (d *defaultRedis) EvalReInt(ctx context.Context, script, key string, args ...interface{}) (int, error) {
    return d.client.Eval(ctx, script, []string{key}, args...).Int()
}
