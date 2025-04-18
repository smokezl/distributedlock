package distributedlock

import (
    "context"
    "fmt"
    "runtime/debug"
    "sync"
    "time"

    "github.com/google/uuid"
)

type DLockGlobal struct {
    dLockI        DistributedI
    retryInterval time.Duration
    autoRenew     bool
    panicHandler  panicHandler
}

type DLock struct {
    g             *DLockGlobal
    key           string
    value         string
    expire        int
    once          sync.Once
    renewInterval time.Duration
    doneChan      chan struct{}
}

func NewDLockGlobal(dl DistributedI, options ...MainOption) *DLockGlobal {
    rl := &DLockGlobal{
        dLockI:        dl,
        retryInterval: defaultRetryInterval,
    }
    // 应用配置函数
    for _, opt := range options {
        opt(rl)
    }
    return rl
}

// 非阻塞模式
func (dlg *DLockGlobal) TryLock(ctx context.Context, key string, ttl int) (dl *DLock, ok bool, err error) {
    val := uuid.New().String()
    if ttl <= 0 {
        ttl = defaultExpire
    }
    ok, err = dlg.dLockI.SetNX(ctx, key, val, ttl)
    if err != nil {
        return
    }
    if !ok {
        return
    }
    dl = &DLock{
        g:      dlg,
        key:    key,
        value:  val,
        expire: ttl,
    }

    if dlg.autoRenew {
        dl.renewInterval = time.Duration(dl.expire) * time.Second / 2
        if dl.renewInterval <= time.Second {
            dl.renewInterval = time.Second
        }
        dl.doneChan = make(chan struct{})
        go dl.dealAutoRenew(ctx)
    }
    return
}

// 阻塞模式
func (dlg *DLockGlobal) Lock(ctx context.Context, key string, ttl, timeout int) (dl *DLock, ok bool, err error) {
    timeoutCh := time.After(time.Duration(timeout) * time.Second)
    for {
        select {
        case <-ctx.Done():
            err = ctx.Err()
            return
        case <-timeoutCh:
            return
        default:
            dl, ok, err = dlg.TryLock(ctx, key, ttl)
            if err != nil {
                return
            }
            if ok {
                return
            }
            time.Sleep(dlg.retryInterval) // 自旋等待
        }
    }
}

func (dlg *DLockGlobal) dealPanicHandler(ctx context.Context, err error) {
    defer func() {
        if e := recover(); e != nil {
            return
        }
    }()
    if dlg.panicHandler != nil {
        dlg.panicHandler(ctx, err)
    }
}

// 自动续期
func (dl *DLock) dealAutoRenew(ctx context.Context) {
    defer func() {
        if e := recover(); e != nil {
            dl.g.dealPanicHandler(ctx, fmt.Errorf("%s dealAutoRenew #panic#:%v,%s",
                name, e, string(debug.Stack()),
            ))
        }
    }()
    ticker := time.NewTicker(dl.renewInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            script := `
                if redis.call("GET", KEYS[1]) == ARGV[1] then
                    return redis.call("EXPIRE", KEYS[1], ARGV[2])
                else
                    return 0
                end`
            _, err := dl.g.dLockI.EvalReInt(ctx, script, dl.key, dl.value, dl.expire)
            if err != nil {
                return
            }
        case <-dl.doneChan:
            return
        }
    }
}

// 解锁
func (dl *DLock) Unlock(ctx context.Context) (err error) {
    if dl.g.autoRenew {
        dl.once.Do(func() {
            close(dl.doneChan) // 停止续期协程
        })
    }
    script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end`
    _, err = dl.g.dLockI.EvalReInt(ctx, script, dl.key, dl.value)
    return err
}
