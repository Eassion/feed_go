package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"enterprise/config"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Client struct {
	RDB *redis.Client
}

func NewRedis(cfg *config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{
		RDB: rdb,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.RDB == nil {
		return nil
	}
	return c.RDB.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.RDB == nil {
		return nil
	}
	return c.RDB.Ping(ctx).Err()
}

func IsMiss(err error) bool {
	return err == redis.Nil
}


//生成一个随机字符串
func randToken(n int) (string, error){
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil{
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	if c == nil || c.RDB == nil {
		return "", false, nil
	}
	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}
	err = c.RDB.SetArgs(ctx, key, token, redis.SetArgs{
		Mode: "NX",
		TTL: ttl,
	}).Err()
	if err != nil {
		return "", false, err
	}
	ok = true
	return token, ok, err
}

var unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)


func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.RDB == nil {
		return nil
	}
	_, err := unlockScript.Run(ctx, c.RDB, []string{key}, token).Result()
	return err
}

