package cache

import "github.com/redis/go-redis/v9"

var client *redis.Client

func NewRedisClient(addr string) {
	client = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func Client() *redis.Client {
	return client
}
