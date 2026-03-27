package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	client *redis.Client
	mu     sync.RWMutex
	config Config
}

type Config struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type Options struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

func NewClient(opts Options) (*Client, error) {
	cfg := redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	client := redis.NewClient(&cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		client: client,
		config: Config{
			Addr:         opts.Addr,
			Password:     opts.Password,
			DB:           opts.DB,
			PoolSize:     opts.PoolSize,
			MinIdleConns: opts.MinIdleConns,
		},
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *Client) Get(ctx context.Context, key string, dest any) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrKeyNotFound
		}
		return err
	}

	return json.Unmarshal(data, dest)
}

func (c *Client) GetString(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Client) SetString(ctx context.Context, key, value string, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *Client) SetInt(ctx context.Context, key string, value int64, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *Client) GetInt(ctx context.Context, key string) (int64, error) {
	return c.client.Get(ctx, key).Int64()
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, key).Result()
}

func (c *Client) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.DecrBy(ctx, key, value).Result()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c.client.Exists(ctx, keys...).Result()
}

func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

func (c *Client) Persist(ctx context.Context, key string) error {
	return c.client.Persist(ctx, key).Err()
}

func (c *Client) Rename(ctx context.Context, key, newkey string) error {
	return c.client.Rename(ctx, key, newkey).Err()
}

func (c *Client) Type(ctx context.Context, key string) (string, error) {
	return c.client.Type(ctx, key).Result()
}

func (c *Client) Append(ctx context.Context, key, value string) (int64, error) {
	return c.client.Append(ctx, key, value).Result()
}

func (c *Client) GetRange(ctx context.Context, key string, start, end int64) (string, error) {
	return c.client.GetRange(ctx, key, start, end).Result()
}

func (c *Client) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	return c.client.SetRange(ctx, key, offset, value).Result()
}

func (c *Client) BitCount(ctx context.Context, key string, bitCount *redis.BitCount) (int64, error) {
	return c.client.BitCount(ctx, key, bitCount).Result()
}

func (c *Client) BitOpAnd(ctx context.Context, destKey string, keys ...string) (int64, error) {
	return c.client.BitOpAnd(ctx, destKey, keys...).Result()
}

func (c *Client) BitOpOr(ctx context.Context, destKey string, keys ...string) (int64, error) {
	return c.client.BitOpOr(ctx, destKey, keys...).Result()
}

func (c *Client) BitOpXor(ctx context.Context, destKey string, keys ...string) (int64, error) {
	return c.client.BitOpXor(ctx, destKey, keys...).Result()
}

func (c *Client) BitOpNot(ctx context.Context, destKey, key string) (int64, error) {
	return c.client.BitOpNot(ctx, destKey, key).Result()
}

func (c *Client) HSet(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.HSet(ctx, key, values...).Result()
}

func (c *Client) HSetNX(ctx context.Context, key, field string, value any) (bool, error) {
	return c.client.HSetNX(ctx, key, field, value).Result()
}

func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return c.client.HDel(ctx, key, fields...).Result()
}

func (c *Client) HExists(ctx context.Context, key, field string) (bool, error) {
	return c.client.HExists(ctx, key, field).Result()
}

func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return c.client.HIncrBy(ctx, key, field, incr).Result()
}

func (c *Client) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	return c.client.HIncrByFloat(ctx, key, field, incr).Result()
}

func (c *Client) HKeys(ctx context.Context, key string) ([]string, error) {
	return c.client.HKeys(ctx, key).Result()
}

func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	return c.client.HLen(ctx, key).Result()
}

func (c *Client) HVals(ctx context.Context, key string) ([]string, error) {
	return c.client.HVals(ctx, key).Result()
}

func (c *Client) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.SAdd(ctx, key, members...).Result()
}

func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	return c.client.SCard(ctx, key).Result()
}

func (c *Client) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.client.SIsMember(ctx, key, member).Result()
}

func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.SMembers(ctx, key).Result()
}

func (c *Client) SPop(ctx context.Context, key string) (string, error) {
	return c.client.SPop(ctx, key).Result()
}

func (c *Client) SRandMember(ctx context.Context, key string, count int) ([]string, error) {
	return c.client.SRandMemberN(ctx, key, int64(count)).Result()
}

func (c *Client) SRem(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.SRem(ctx, key, members...).Result()
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	return c.client.ZAdd(ctx, key, members...).Result()
}

func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	return c.client.ZCard(ctx, key).Result()
}

func (c *Client) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return c.client.ZCount(ctx, key, min, max).Result()
}

func (c *Client) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return c.client.ZIncrBy(ctx, key, increment, member).Result()
}

func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRange(ctx, key, start, stop).Result()
}

func (c *Client) ZRank(ctx context.Context, key, member string) (int64, error) {
	return c.client.ZRank(ctx, key, member).Result()
}

func (c *Client) ZRem(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.ZRem(ctx, key, members...).Result()
}

func (c *Client) ZScore(ctx context.Context, key, member string) (float64, error) {
	return c.client.ZScore(ctx, key, member).Result()
}

func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRevRange(ctx, key, start, stop).Result()
}

func (c *Client) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	return c.client.ZRevRank(ctx, key, member).Result()
}

func (c *Client) LPush(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.LPush(ctx, key, values...).Result()
}

func (c *Client) RPush(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.RPush(ctx, key, values...).Result()
}

func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	return c.client.LPop(ctx, key).Result()
}

func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	return c.client.RPop(ctx, key).Result()
}

func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	return c.client.LLen(ctx, key).Result()
}

func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.LRange(ctx, key, start, stop).Result()
}

func (c *Client) LTrim(ctx context.Context, key string, start, stop int64) error {
	return c.client.LTrim(ctx, key, start, stop).Err()
}

func (c *Client) PFAdd(ctx context.Context, key string, els ...any) (int64, error) {
	return c.client.PFAdd(ctx, key, els...).Result()
}

func (c *Client) PFCount(ctx context.Context, keys ...string) (int64, error) {
	return c.client.PFCount(ctx, keys...).Result()
}

func (c *Client) PubSub(ctx context.Context) *redis.PubSub {
	return c.client.Subscribe(ctx)
}

func (c *Client) Publish(ctx context.Context, channel string, message any) (int64, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return 0, err
	}
	return c.client.Publish(ctx, channel, data).Result()
}

func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

func (c *Client) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

func (c *Client) TxPipeline() redis.Pipeliner {
	return c.client.TxPipeline()
}

func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	return c.client.Scan(ctx, cursor, match, count).Result()
}

func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.client.Keys(ctx, pattern).Result()
}

func (c *Client) FlushDB(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

func (c *Client) FlushAll(ctx context.Context) error {
	return c.client.FlushAll(ctx).Err()
}

func (c *Client) DBSize(ctx context.Context) (int64, error) {
	return c.client.DBSize(ctx).Result()
}

func (c *Client) Info(ctx context.Context) (string, error) {
	return c.client.Info(ctx).Result()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

type MultiClient struct {
	clients []*Client
}

func NewMultiClient(clients ...*Client) *MultiClient {
	return &MultiClient{clients: clients}
}

func (m *MultiClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	for _, client := range m.clients {
		if err := client.Set(ctx, key, value, expiration); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiClient) Get(ctx context.Context, key string, dest any) error {
	for _, client := range m.clients {
		if err := client.Get(ctx, key, dest); err == nil {
			return nil
		}
	}
	return ErrKeyNotFound
}

func (m *MultiClient) Del(ctx context.Context, keys ...string) error {
	for _, client := range m.clients {
		client.Del(ctx, keys...)
	}
	return nil
}

var ErrKeyNotFound = fmt.Errorf("key not found")

func init() {}
