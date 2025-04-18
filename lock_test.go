package distributedlock

import (
    "context"
    "fmt"
    "sync"
    "testing"
    "time"

    "github.com/redis/go-redis/v9"
)

var (
    customRedis *CustomRedis

    defaultLock *DLockGlobal
    customLock  *DLockGlobal
)

var conf = &RedisConfig{
    Addr:        "127.0.0.1:6379",
    MaxRetry:    2,
    Pwd:         "",
    IdleTimeout: 10,
    ConnTimeout: 100,
    MaxIdle:     50,
    MaxActive:   500,
}

type CustomRedis struct {
    client *redis.Client
}

func NewCustomRedis(c *RedisConfig) *CustomRedis {
    return &CustomRedis{
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

func (c *CustomRedis) SetNX(ctx context.Context, key, value string, ttl int) (bool, error) {
    return c.client.SetNX(ctx, key, value, time.Duration(ttl)*time.Second).Result()
}

func (c *CustomRedis) EvalReInt(ctx context.Context, script, key string, args ...interface{}) (int, error) {
    return c.client.Eval(ctx, script, []string{key}, args...).Int()
}

func TestMain(m *testing.M) {
    customRedis = NewCustomRedis(conf)
    defaultLock = NewDLockGlobal(UseDefaultRedis(conf),
        WithPanicHandler(func(ctx context.Context, err error) {
            fmt.Println("default err====>", err)
        }),
        WithAutoRenew(),
    )
    customLock = NewDLockGlobal(customRedis)
    m.Run()
}

// 示例用法
func TestNonBlocking(t *testing.T) {
    // 非阻塞
    key := "test_lock_nb"
    ctx := context.Background()
    lock, ok, _ := defaultLock.TryLock(ctx, key, 10)
    if ok {
        time.Sleep(time.Second * 2)
        fmt.Println("++加锁后key值", customRedis.client.Get(ctx, key))
        fmt.Println("++加锁后key TTL", customRedis.client.TTL(ctx, key))
        lock.Unlock(ctx)
        fmt.Println("--解锁后key值", customRedis.client.Get(ctx, key))
        fmt.Println("--解锁后key TTL", customRedis.client.TTL(ctx, key))
    } else {
        fmt.Println("加锁失败")
    }
}

func TestNonBlockingRace(t *testing.T) {
    // 非阻塞竞争
    key := "test_lock_nbr"
    ctx := context.Background()
    lock, ok, _ := defaultLock.TryLock(ctx, key, 10)
    if ok {
        go func() {
            fmt.Println("++加锁后key值", customRedis.client.Get(ctx, key))
            fmt.Println("++加锁后key TTL", customRedis.client.TTL(ctx, key))
            time.Sleep(time.Second * 3)
            lock.Unlock(ctx)
            fmt.Println("--解锁后key值", customRedis.client.Get(ctx, key))
            fmt.Println("--解锁后key TTL", customRedis.client.TTL(ctx, key))
        }()
    } else {
        fmt.Println("加锁失败")
    }
    lock1, ok, _ := defaultLock.TryLock(ctx, key, 10)
    if ok {
        fmt.Println("竞争锁 1 ++加锁成功")
        lock1.Unlock(ctx)
    } else {
        fmt.Println("竞争锁 1 ++加锁失败")
    }
    time.Sleep(time.Second * 5)

    lock2, ok, _ := defaultLock.TryLock(ctx, key, 10)
    if ok {
        fmt.Println("竞争锁 2 ++加锁成功")
        fmt.Println("竞争锁 2 ++加锁后key值", customRedis.client.Get(ctx, key))
        fmt.Println("竞争锁 2 ++加锁后key TTL", customRedis.client.TTL(ctx, key))
        lock2.Unlock(ctx)
    } else {
        fmt.Println("竞争锁 2 ++加锁失败")
    }
}

func TestNonBlockingAutoRenew(t *testing.T) {
    // 非阻塞续期
    key := "test_lock_nbar"
    ctx := context.Background()
    lock, ok, _ := defaultLock.TryLock(ctx, key, 5)
    if ok {
        val := customRedis.client.Get(ctx, key)
        fmt.Println("++加锁后key值", val)
        fmt.Println("++续期执行间隔", lock.renewInterval)
        for i := 0; i < 10; i++ {
            compareVal := customRedis.client.Get(ctx, key)
            if val.Val() != compareVal.Val() {
                fmt.Println("！！！！！！锁 value 发生变化，原：", val.Val(), "，新：", compareVal.Val())
                return
            }
            fmt.Println("++加锁后key TTL", customRedis.client.TTL(ctx, key))
            time.Sleep(time.Second * 1)
        }
        lock.Unlock(ctx)
        fmt.Println("--解锁后key值", customRedis.client.Get(ctx, key))
        fmt.Println("--解锁后key TTL", customRedis.client.TTL(ctx, key))
    }
}

func TestBlocking(t *testing.T) {
    // 阻塞
    key := "test_lock_nbar"
    ctx := context.Background()
    sleepTime := time.Second * 3
    var wg sync.WaitGroup
    wg.Add(3)
    go func() {
        defer wg.Done()
        lock, ok, err := defaultLock.Lock(ctx, key, 10, 5)
        if err != nil {
            fmt.Println("协程1报错", err)
            return
        }
        if ok {
            defer lock.Unlock(ctx)
            fmt.Println("协程1获取锁")
            time.Sleep(sleepTime)
        } else {
            fmt.Println("协程1获取锁失败")
        }
    }()
    go func() {
        defer wg.Done()
        lock, ok, err := defaultLock.Lock(ctx, key, 10, 5)
        if err != nil {
            fmt.Println("协程2报错", err)
            return
        }
        if ok {
            defer lock.Unlock(ctx)
            fmt.Println("协程2获取锁")
            time.Sleep(sleepTime)
        } else {
            fmt.Println("协程2获取锁失败")
        }
    }()

    go func() {
        defer wg.Done()
        lock, ok, err := defaultLock.Lock(ctx, key, 10, 5)
        if err != nil {
            fmt.Println("协程3报错", err)
            return
        }
        if ok {
            defer lock.Unlock(ctx)
            fmt.Println("协程3获取锁")
            time.Sleep(sleepTime)
        } else {
            fmt.Println("协程3获取锁失败")
        }
    }()
    wg.Wait()
}

func TestBlockingExpiredDelete(t *testing.T) {
    // 模拟 a 锁过期，b 获取到锁后，a 是否能删掉 b 的锁
    key := "test_lock_bed"
    ctx := context.Background()
    lock, ok, _ := defaultLock.TryLock(ctx, key, 5)
    if ok {
        go func() {
            fmt.Println("++加锁后key值", customRedis.client.Get(ctx, key))
            fmt.Println("++加锁后key TTL", customRedis.client.TTL(ctx, key))
            time.Sleep(time.Second * 2)
            fmt.Println("原锁触发删除")
            lock.Unlock(ctx)
            time.Sleep(time.Second * 2)
            // 继续删除
            fmt.Println("原锁触发删除")
            lock.Unlock(ctx)
            time.Sleep(time.Second * 2)
            fmt.Println("原锁触发删除")
            lock.Unlock(ctx)
        }()
    } else {
        fmt.Println("加锁失败")
    }
    lock1, ok, _ := defaultLock.Lock(ctx, key, 5, 10)
    if ok {
        fmt.Println("竞争锁 1 ++加锁成功")
        for i := 0; i < 5; i++ {
            fmt.Println("竞争锁 1 ++加锁后key值", customRedis.client.Get(ctx, key))
            fmt.Println("竞争锁 1 ++加锁后key TTL", customRedis.client.TTL(ctx, key))
            time.Sleep(time.Second * 2)
        }
        lock1.Unlock(ctx)
        fmt.Println("竞争锁 1 --解锁后key值", customRedis.client.Get(ctx, key))
        fmt.Println("竞争锁 1 --解锁后key TTL", customRedis.client.TTL(ctx, key))
    } else {
        fmt.Println("竞争锁 1 ++加锁失败")
    }
}
