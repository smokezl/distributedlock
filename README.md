go 分布式锁
===========
### go 分布式锁，默认基于 redis，可依赖注入。

### 功能
* 支持带超时时间的阻塞获取
* 支持非阻塞获取
* 支持锁续期
* 利用lua进行原子删除，防止操作未完成但锁已过期时，可能发生的锁交叉持有者误删问题，保证锁释放的安全性
* 支持自定义错误处理，WithPanicHandler

### 安装
go get github.com/smokezl/distributedlock

### 导入
import "github.com/smokezl/distributedlock"

### 基本使用方式
#### 初始化
 ```go
// 1、初始化全局对象
conf := &distributedlock.RedisConfig{
    Addr:        "127.0.0.1:6379",
    MaxRetry:    2,
    Pwd:         "",
    IdleTimeout: 10,
    ConnTimeout: 100,
    MaxIdle:     50,
    MaxActive:   500,
}

globalDLock = distributedlock.NewDLockGlobal(distributedlock.UseDefaultRedis(conf),
    WithPanicHandler(func(ctx context.Context, err error) {
        fmt.Println("default err====>", err)
    }),
    WithAutoRenew(),
)

// 2、根据不同的加锁场景，获取分布式锁
key := "lock_scene_1"
ttl := 10
ctx := context.Background()
lock, ok, err := globalDLock.TryLock(ctx, key, ttl)
if err != nil {
    // 处理 err 情况
}
if ok {
    defer lock.Unlock(ctx)
    // 获取锁成功，处理业务逻辑
} else {
    // 获取锁失败处理逻辑
}

// 3、阻塞式获取分布式锁
key := "lock_scene_2"
ttl := 10
timeout := 5
ctx := context.Background()
lock, ok, err := globalDLock.Lock(ctx, key, ttl, timeout)
if err != nil {
    // 处理 err 情况
}
if ok {
    defer lock.Unlock(ctx)
    // 获取锁成功，处理业务逻辑
} else {
    // 获取锁失败处理逻辑
}
```

#### 初始化全局对象配置函数
##### 1、WithAutoRenew() 
是否设置自动锁续期，默认为：否，调用此方法后，可以进行锁续期

##### 2、WithPanicHandler(f panicHandler) 

设置 panic 异常执行函数，默认为 nil
```go
type panicHandler func(ctx context.Context, err error)
```
##### 3、DistributedI

设置从redis缓存io数据的接口，内置 UseDefaultRedis

```go
type DistributedI interface {
    // SetNX ttl 单位：秒
    SetNX(ctx context.Context, key, value string, ttl int) (bool, error)
    EvalReInt(ctx context.Context, script, key string, args ...interface{}) (int, error)
}
```
如果不使用内置 UseDefaultRedis，需要进行手动注入，具体实现方法为
```go
type CustomRedis struct {
    // 这里的 redisInstance 是业务中的 redis 实例
    client *redisInstance
}

func NewCustomRedis(client *redisInstance) *CustomRedis {
	return &CustomRedis{
        client: client
	}
}

func (c *CustomRedis) SetNX(ctx context.Context, key, value string, ttl int) (bool, error) {
    return c.client.SetNX(ctx, key, value, time.Duration(ttl)*time.Second).Result()
}

func (c *CustomRedis) EvalReInt(ctx context.Context, script, key string, args ...interface{}) (int, error) {
    return c.client.Eval(ctx, script, []string{key}, args...).Int()
}

client := 自定义 redis client
customGlobalDLock = distributedlock.NewDLockGlobal(NewCustomRedis(client))
```

MIT licence.