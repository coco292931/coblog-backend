package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// 这些测试依赖本机 Redis，不可用时自动跳过。
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()

	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:6379",
		DialTimeout: 500 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		t.Skipf("本机 Redis 不可用，跳过: %v", err)
	}
	return c
}

func testKey(t *testing.T, name string) string {
	t.Helper()
	return "coblog:test:" + name + ":" + time.Now().Format("150405.000000")
}

func TestTakeIsOneShot(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "take")
	defer c.Del(ctx, key)

	if err := store.Set(ctx, key, "payload-1", time.Minute); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	got, hit, err := store.Take(ctx, key)
	if err != nil {
		t.Fatalf("取出失败: %v", err)
	}
	if !hit || got != "payload-1" {
		t.Fatalf("应取出 payload-1，实际 hit=%v got=%q", hit, got)
	}

	// 第二次应已被消费（一次性）
	if _, hit, err := store.Take(ctx, key); err != nil || hit {
		t.Fatalf("重复取出应当失败，实际 hit=%v err=%v", hit, err)
	}
}

func TestTakeIfMatchRequiresExactValue(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "takeifmatch")
	defer c.Del(ctx, key)

	if err := store.Set(ctx, key, "123456", time.Minute); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 值不匹配时不应取出，且记录必须保留
	if _, hit, err := store.TakeIfMatch(ctx, key, "000000"); err != nil || hit {
		t.Fatalf("错误值不应取出，实际 hit=%v err=%v", hit, err)
	}
	if v, hit, _ := store.Get(ctx, key); !hit || v != "123456" {
		t.Fatal("校验失败后记录必须保留，否则暴力枚举可直接打掉记录")
	}

	// 匹配时才取出并删除
	if v, hit, err := store.TakeIfMatch(ctx, key, "123456"); err != nil || !hit || v != "123456" {
		t.Fatalf("正确值应取出，实际 hit=%v v=%q err=%v", hit, v, err)
	}
	if _, hit, _ := store.Get(ctx, key); hit {
		t.Fatal("取出后记录应被删除")
	}
}

func TestTakeExpires(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "expire")
	defer c.Del(ctx, key)

	if err := store.Set(ctx, key, "payload", 50*time.Millisecond); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	if _, hit, err := store.Take(ctx, key); err != nil || hit {
		t.Fatalf("过期后不应取出内容，实际 hit=%v err=%v", hit, err)
	}
}

func TestAcquireCooldown(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "cooldown")
	defer c.Del(ctx, key)

	if !store.AcquireCooldown(ctx, key, time.Minute) {
		t.Fatal("首次获取冷却应成功")
	}
	if store.AcquireCooldown(ctx, key, time.Minute) {
		t.Fatal("冷却期内不应再次获取成功")
	}

	// 释放后应可再次获取（对应「邮件发送失败允许立即重试」）
	if err := store.ReleaseCooldown(ctx, key); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if !store.AcquireCooldown(ctx, key, time.Minute) {
		t.Fatal("释放后应可重新获取")
	}
}

func TestCooldownExpires(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "cooldownexpire")
	defer c.Del(ctx, key)

	if !store.AcquireCooldown(ctx, key, 50*time.Millisecond) {
		t.Fatal("首次获取应成功")
	}
	time.Sleep(150 * time.Millisecond)
	if !store.AcquireCooldown(ctx, key, time.Minute) {
		t.Fatal("冷却过期后应可再次获取")
	}
}

func TestGetSetJSON(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	key := testKey(t, "json")
	defer c.Del(ctx, key)

	payload := map[string]string{"feed": "<rss>中文 & 符号</rss>"}
	if err := store.SetJSON(ctx, key, payload, time.Minute); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	var got map[string]string
	hit, err := store.GetJSON(ctx, key, &got)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if !hit || got["feed"] != payload["feed"] {
		t.Fatalf("读取结果不符: hit=%v got=%v", hit, got)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	var miss map[string]string
	if hit, _ := store.GetJSON(ctx, key, &miss); hit {
		t.Fatal("删除后应未命中")
	}
}

func TestDeleteByPrefix(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	store := NewStore(c)
	prefix := "coblog:test:prefix:" + time.Now().Format("150405.000000") + ":"
	// 前缀相近但不应被误删的键
	sibling := "coblog:test:prefix:" + time.Now().Format("150405.000000") + "x:keep"

	defer func() {
		store.DeleteByPrefix(ctx, prefix)
		c.Del(ctx, sibling)
	}()

	for _, k := range []string{prefix + "a", prefix + "b"} {
		if err := store.Set(ctx, k, "x", time.Minute); err != nil {
			t.Fatalf("写入 %s 失败: %v", k, err)
		}
	}
	if err := store.Set(ctx, sibling, "x", time.Minute); err != nil {
		t.Fatalf("写入对照键失败: %v", err)
	}

	if err := store.DeleteByPrefix(ctx, prefix); err != nil {
		t.Fatalf("按前缀删除失败: %v", err)
	}

	for _, k := range []string{prefix + "a", prefix + "b"} {
		if _, hit, _ := store.Get(ctx, k); hit {
			t.Errorf("%s 应被删除", k)
		}
	}
	if _, hit, _ := store.Get(ctx, sibling); !hit {
		t.Error("前缀之外的键不应被删除")
	}
}

func TestNewTokenIsUniqueAndURLSafe(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		token, err := NewToken()
		if err != nil {
			t.Fatalf("生成令牌失败: %v", err)
		}
		if token == "" {
			t.Fatal("令牌不应为空")
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("令牌重复: %s", token)
		}
		seen[token] = struct{}{}

		// base64url 不应包含需要转义的字符
		for _, ch := range token {
			if ch == '+' || ch == '/' || ch == '=' {
				t.Fatalf("令牌含 URL 不安全字符: %s", token)
			}
		}
	}
}

func TestUnavailableStoreDegradesGracefully(t *testing.T) {
	// Redis 不可用时所有操作都不应 panic：读取返回未命中，冷却不放行，写入报错
	store := NewStore(nil)
	ctx := context.Background()

	if store.Available() {
		t.Fatal("nil 客户端时不应报告可用")
	}
	if _, hit, err := store.Take(ctx, "k"); hit || err != nil {
		t.Errorf("不可用时应未命中且无错误，实际 hit=%v err=%v", hit, err)
	}
	if _, hit, err := store.TakeIfMatch(ctx, "k", "v"); hit || err != nil {
		t.Errorf("不可用时应未命中且无错误，实际 hit=%v err=%v", hit, err)
	}
	if store.AcquireCooldown(ctx, "k", time.Minute) {
		t.Error("不可用时不应对冷却放行")
	}
	if hit, err := store.GetJSON(ctx, "k", &struct{}{}); hit || err != nil {
		t.Errorf("不可用时应未命中且无错误，实际 hit=%v err=%v", hit, err)
	}
	if err := store.Delete(ctx, "k"); err != nil {
		t.Errorf("不可用时删除应为无操作，实际 %v", err)
	}
}

func TestUnavailableSetReturnsErrUnavailable(t *testing.T) {
	store := NewStore(nil)
	if err := store.Set(context.Background(), "k", "v", time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Errorf("不可用时写入应返回 ErrUnavailable，实际 %v", err)
	}
}

func TestKeyJoinsWithColon(t *testing.T) {
	if got := Key("coblog", "activation", "abc"); got != "coblog:activation:abc" {
		t.Errorf("Key 拼接结果错误: %s", got)
	}
}

var _ = json.Marshal
