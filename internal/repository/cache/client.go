package cache

import "github.com/redis/go-redis/v9"

var client *redis.Client

func NewRedisClient() {
	client = redis.NewClient(&redis.Options{})
}

func Client() *redis.Client {
	return client
}
