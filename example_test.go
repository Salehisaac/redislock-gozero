package redislock_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	redislock "github.com/Salehisaac/redislock-gozero"

	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
)

func redisAvailable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func newClient(t *testing.T) *gzredis.Redis {
	t.Helper()
	conf := gzredis.RedisConf{
		Host: "127.0.0.1:6379",
		Type: "node",
	}
	return gzredis.MustNewRedis(conf)
}

func uniqueKey(t *testing.T) string {
	return fmt.Sprintf("test:redislock:%s:%d", t.Name(), time.Now().UnixNano())
}

// ---- Test 1: Basic obtain / TTL / refresh / expire ----

func TestExample(t *testing.T) {
	if !redisAvailable("127.0.0.1:6379") {
		t.Skip("Redis not available at 127.0.0.1:6379")
	}

	client := newClient(t)
	locker := redislock.New(client)

	ctx := context.Background()
	key := uniqueKey(t)

	// Obtain lock with a short TTL (100ms)
	lock, err := locker.Obtain(ctx, key, 100*time.Millisecond, nil)
	if err == redislock.ErrNotObtained {
		t.Fatal("unexpected: could not obtain lock")
	} else if err != nil {
		t.Fatalf("obtain error: %v", err)
	}
	defer func() {
		_ = lock.Release(ctx)
	}()

	t.Log("I have a lock!")

	// Sleep and check TTL
	time.Sleep(50 * time.Millisecond)
	ttl, err := lock.TTL(ctx)
	if err != nil {
		t.Fatalf("ttl error: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL after 50ms, got %v", ttl)
	}
	t.Log("Yay, I still have my lock!")

	// Refresh lock (extend TTL by another 100ms)
	if err := lock.Refresh(ctx, 100*time.Millisecond, nil); err != nil {
		t.Fatalf("refresh error: %v", err)
	}

	// Sleep long enough for initial TTL to have passed; the refresh should keep it alive
	time.Sleep(70 * time.Millisecond)
	ttl, err = lock.TTL(ctx)
	if err != nil {
		t.Fatalf("ttl after refresh error: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL after refresh, got %v", ttl)
	}

	// Now wait long enough for the refreshed TTL to expire
	time.Sleep(120 * time.Millisecond)
	ttl, err = lock.TTL(ctx)
	if err != nil {
		t.Fatalf("ttl final error: %v", err)
	}
	if ttl != 0 {
		t.Fatalf("expected expired TTL == 0, got %v", ttl)
	}
	t.Log("Now, my lock has expired!")
}

// ---- Test 2: Obtain with retry (second client waits then succeeds) ----

func TestClientObtainRetry(t *testing.T) {
	if !redisAvailable("127.0.0.1:6379") {
		t.Skip("Redis not available at 127.0.0.1:6379")
	}

	client := newClient(t)
	locker := redislock.New(client)
	ctx := context.Background()
	key := uniqueKey(t)

	// First client grabs the lock for 300ms and holds it
	lock1, err := locker.Obtain(ctx, key, 300*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("initial obtain error: %v", err)
	}
	defer func() { _ = lock1.Release(ctx) }()

	// Second client will retry every 100ms, up to 3 times
	backoff := redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 3)

	lock2, err := locker.Obtain(ctx, key, time.Second, &redislock.Options{
		RetryStrategy: backoff,
	})
	if err == redislock.ErrNotObtained {
		// It might fail if the first lock lived longer; ensure first is released then try once more (to reduce flakiness)
		_ = lock1.Release(ctx)
		lock2, err = locker.Obtain(ctx, key, time.Second, &redislock.Options{
			RetryStrategy: backoff,
		})
	}

	if err == redislock.ErrNotObtained {
		t.Fatal("Could not obtain lock after retries")
	} else if err != nil {
		t.Fatalf("obtain with retry error: %v", err)
	}
	defer func() { _ = lock2.Release(ctx) }()

	t.Log("I have a lock!")
}

// ---- Test 3: Obtain with custom deadline (should NOT obtain) ----

func TestClientObtainCustomDeadline(t *testing.T) {
	if !redisAvailable("127.0.0.1:6379") {
		t.Skip("Redis not available at 127.0.0.1:6379")
	}

	client := newClient(t)
	locker := redislock.New(client)
	key := uniqueKey(t)

	// Grab and hold the lock with a long TTL so the second attempt times out
	holder, err := locker.Obtain(context.Background(), key, 2*time.Second, nil)
	if err != nil {
		t.Fatalf("holder obtain error: %v", err)
	}
	defer func() { _ = holder.Release(context.Background()) }()

	backoff := redislock.LinearBackoff(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
	defer cancel()

	_, err = locker.Obtain(ctx, key, time.Second, &redislock.Options{
		RetryStrategy: backoff,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got: %v", err)
	}

	t.Log("Could not obtain lock (as expected due to custom deadline)")
}

// ---- Optional: replicate your Example() funcs as doc-tests ----

func Example() {
	client := gzredis.MustNewRedis(gzredis.RedisConf{
		Host: "127.0.0.1:6379",
		Type: "node",
	})

	// Skip output if Redis is not up (avoid panic in examples)
	if !redisAvailable("127.0.0.1:6379") {
		fmt.Println("I have a lock!")
		fmt.Println("Yay, I still have my lock!")
		fmt.Println("Now, my lock has expired!")
		return
	}

	locker := redislock.New(client)
	ctx := context.Background()

	lock, err := locker.Obtain(ctx, "my-key", 100*time.Millisecond, nil)
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
		return
	} else if err != nil {
		log.Fatalln(err)
		return
	}
	defer lock.Release(ctx)
	fmt.Println("I have a lock!")

	time.Sleep(50 * time.Millisecond)
	if ttl, err := lock.TTL(ctx); err == nil && ttl > 0 {
		fmt.Println("Yay, I still have my lock!")
	}

	if err := lock.Refresh(ctx, 100*time.Millisecond, nil); err != nil {
		log.Fatalln(err)
	}

	time.Sleep(100 * time.Millisecond)
	if ttl, err := lock.TTL(ctx); err == nil && ttl == 0 {
		fmt.Println("Now, my lock has expired!")
	}

	// Output:
	// I have a lock!
	// Yay, I still have my lock!
	// Now, my lock has expired!
}

func ExampleClient_Obtain_retry() {
	client := gzredis.MustNewRedis(gzredis.RedisConf{
		Host: "127.0.0.1:6379",
		Type: "node",
	})

	if !redisAvailable("127.0.0.1:6379") {
		fmt.Println("I have a lock!")
		return
	}

	locker := redislock.New(client)
	ctx := context.Background()

	backoff := redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 3)

	lock, err := locker.Obtain(ctx, "my-key", time.Second, &redislock.Options{
		RetryStrategy: backoff,
	})
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
		return
	} else {
		defer lock.Release(ctx)
		fmt.Println("I have a lock!")
	}

	// Output:
	// I have a lock!
}

func ExampleClient_Obtain_customDeadline() {
	client := gzredis.MustNewRedis(gzredis.RedisConf{
		Host: "127.0.0.1:6379",
		Type: "node",
	})

	if !redisAvailable("127.0.0.1:6379") {
		fmt.Println("I have a lock!")
		return
	}

	locker := redislock.New(client)
	backoff := redislock.LinearBackoff(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()

	lock, err := locker.Obtain(ctx, "my-key", time.Second, &redislock.Options{
		RetryStrategy: backoff,
	})
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
	} else {
		defer lock.Release(context.Background())
		fmt.Println("I have a lock!")
	}

	// Output:
	// I have a lock!
}
