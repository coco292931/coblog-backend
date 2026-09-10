package cache

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrUnavailable 表示 Redis 不可用，调用方应据此降级或明确报错
var ErrUnavailable = errors.New("cache: redis 不可用")

// Store 是对 Redis 的薄封装，提供本项目需要的原语。
// 当底层客户端为 nil（未配置或连接失败）时所有操作安全降级：
// 读取类返回未命中，冷却获取返回 false，写入类返回 ErrUnavailable。
type Store struct {
	client *redis.Client
}

// NewStore 构造 Store；client 允许为 nil，此时全部操作降级
func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

// Available 报告底层 Redis 是否可用
func (s *Store) Available() bool {
	return s != nil && s.client != nil
}

// ---- 基础读写 ----

// Set 写入字符串值并设置有效期，ttl <= 0 表示不设置过期
func (s *Store) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}
	return s.client.Set(ctx, key, value, ttl).Err()
}

// Get 读取字符串值，未命中时 hit 为 false 且 err 为 nil
func (s *Store) Get(ctx context.Context, key string) (value string, hit bool, err error) {
	if !s.Available() {
		return "", false, nil
	}
	value, err = s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Delete 删除指定键
func (s *Store) Delete(ctx context.Context, key string) error {
	if !s.Available() {
		return nil
	}
	return s.client.Del(ctx, key).Err()
}

// SetJSON 以 JSON 形式写入值
func (s *Store) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.Set(ctx, key, string(raw), ttl)
}

// GetJSON 读取并反序列化 JSON 值，未命中时 hit 为 false 且 err 为 nil
func (s *Store) GetJSON(ctx context.Context, key string, out interface{}) (hit bool, err error) {
	raw, hit, err := s.Get(ctx, key)
	if !hit || err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		// 值损坏视作未命中，避免影响主流程
		return false, nil
	}
	return true, nil
}

// ---- 原子消费（本机 Redis 5 无 GETDEL，统一用 Lua 保证原子性）----

// takeScript 原子取出并删除键值，键不存在返回 nil
var takeScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return nil end
redis.call('DEL', KEYS[1])
return v
`)

// takeIfMatchScript 仅当当前值等于期望值时才原子取出并删除。
// 用于验证码这类低熵场景：只有猜中才消耗记录，避免暴力枚举把记录直接打掉；
// 同时保证「校验通过即失效」不存在竞态下的重复消费。
var takeIfMatchScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return nil end
if v ~= ARGV[1] then return nil end
redis.call('DEL', KEYS[1])
return v
`)

// Take 原子取出并删除键（一次性消费），未命中或不可用时 hit 为 false
func (s *Store) Take(ctx context.Context, key string) (value string, hit bool, err error) {
	if !s.Available() {
		return "", false, nil
	}
	res, err := takeScript.Run(ctx, s.client, []string{key}).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	value, ok := res.(string)
	if !ok {
		return "", false, nil
	}
	return value, true, nil
}

// TakeIfMatch 仅当键值等于 expected 时原子取出并删除
func (s *Store) TakeIfMatch(ctx context.Context, key, expected string) (value string, hit bool, err error) {
	if !s.Available() {
		return "", false, nil
	}
	res, err := takeIfMatchScript.Run(ctx, s.client, []string{key}, expected).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	value, ok := res.(string)
	if !ok {
		return "", false, nil
	}
	return value, true, nil
}

// ---- 频率限制 ----

// AcquireCooldown 以 SET NX EX 语义抢占冷却标记，返回 true 表示抢到（此前不存在）
func (s *Store) AcquireCooldown(ctx context.Context, key string, ttl time.Duration) bool {
	if !s.Available() {
		return false
	}
	ok, err := s.client.SetNX(ctx, key, "1", ttl).Result()
	return err == nil && ok
}

// ReleaseCooldown 释放冷却标记，用于「发送失败后允许立即重试」的场景
func (s *Store) ReleaseCooldown(ctx context.Context, key string) error {
	return s.Delete(ctx, key)
}

// ---- 批量失效 ----

// DeleteByPrefix 按前缀删除键（使用 SCAN，避免 KEYS 阻塞）。
// 仅用于缓存失效，不保证与其他写操作的事务一致性。
func (s *Store) DeleteByPrefix(ctx context.Context, prefix string) error {
	if !s.Available() {
		return nil
	}

	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// ---- 工具 ----

// NewToken 生成 24 字节高熵随机令牌，base64url 编码后可安全放入 URL
func NewToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Key 拼接 Redis 键，便于按命名空间组织与批量清理
func Key(parts ...string) string {
	return strings.Join(parts, ":")
}
