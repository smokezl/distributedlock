package distributedlock

import (
    "context"
    "time"
)

var (
    name = "distributed lock"
    // 默认锁时间，单位：秒
    defaultExpire = 30
    // 默认自动续期间隔，单位：毫秒
    defaultRetryInterval = 300 * time.Millisecond
)

type panicHandler func(ctx context.Context, err error)

type MainOption func(*DLockGlobal)

// WithAutoRenew 设置是否自动续期
func WithAutoRenew() MainOption {
    return func(s *DLockGlobal) {
        s.autoRenew = true
    }
}

// WithPanicHandler 异常 panic 处理
func WithPanicHandler(f panicHandler) MainOption {
    return func(do *DLockGlobal) {
        do.panicHandler = f
    }
}

type DistributedI interface {
    // SetNX ttl 单位：秒
    SetNX(ctx context.Context, key, value string, ttl int) (bool, error)
    EvalReInt(ctx context.Context, script, key string, args ...interface{}) (int, error)
}
