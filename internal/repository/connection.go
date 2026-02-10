package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/service"
	"github.com/redis/go-redis/v9"
)

type ConnectionRepo struct {
	redis *redis.Client
}

func NewConnectionRepo(redis *redis.Client) *ConnectionRepo {
	return &ConnectionRepo{
		redis: redis,
	}
}

func (cr *ConnectionRepo) Online(ctx context.Context, userID int) error {
	key := fmt.Sprintf("online:%d", userID)
	return cr.redis.Set(ctx, key, true, 80*time.Second).Err()
}

func (cr *ConnectionRepo) IsOnline(ctx context.Context, userID int) (bool, error) {
	key := fmt.Sprintf("online:%d", userID)
	v, err := cr.redis.Get(ctx, key).Bool()
	if err == redis.Nil {
		return false, nil
	}
	return v, nil

}

func (cr *ConnectionRepo) Subscribe(ctx context.Context, userID int) *redis.PubSub {
	channel := fmt.Sprintf("message:%d", userID)
	return cr.redis.Subscribe(ctx, channel)
}

func (cr *ConnectionRepo) Produce(ctx context.Context, channel string, msg *service.ProduceMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return cr.redis.Publish(ctx, channel, data).Err()
}
