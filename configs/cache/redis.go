package cache

import (
	"context"
	"log"
	"sync"
	"time"

	configreader "coblog-backend/configs/configReader"

	"github.com/redis/go-redis/v9"
)

// Client 全局 Redis 客户端。Init 未被调用或 Redis 不可用时为 nil，
// 此时 Available() 返回 false，调用方应降级到无缓存逻辑。
var Client *redis.Client

var once sync.Once

// Init 初始化 Redis 连接（幂等）。连接失败不会终止进程：
// 记录警告并保持 Client 为 nil，由调用方降级处理，避免 Redis 故障导致整个服务不可用。
func Init() {
	once.Do(func() {
		cfg := configreader.GetConfig().Redis
		if cfg.Host == "" {
			log.Printf("[WARN][Redis] 未配置 redis.host，缓存功能已禁用")
			return
		}

		c := redis.NewClient(&redis.Options{
			Addr:        cfg.Addr(),
			Password:    cfg.Password,
			DB:          cfg.DB,
			DialTimeout: 2 * time.Second,
			ReadTimeout: 2 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := c.Ping(ctx).Err(); err != nil {
			log.Printf("[WARN][Redis] 连接失败，缓存功能已禁用: %v", err)
			_ = c.Close()
			return
		}

		Client = c
		log.Printf("[INFO][Redis] 连接成功: %s db=%d", cfg.Addr(), cfg.DB)
	})
}

// Available 报告 Redis 是否可用
func Available() bool {
	return Client != nil
}

// Close 关闭连接，供程序退出时调用
func Close() error {
	if Client == nil {
		return nil
	}
	return Client.Close()
}
