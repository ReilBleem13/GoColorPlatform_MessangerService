package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// Online V2
func (cr *ConnectionRepo) UpdateOnlineStatus(ctx context.Context, in *service.Presence) error {
	key := fmt.Sprintf("user:online:%d", in.UserID)
	return cr.redis.Set(ctx, key, in.Timestamp, 3*24*time.Hour).Err()
}

func (cr *ConnectionRepo) GetOnlineStatus(ctx context.Context, userID int) (time.Time, error) {
	key := fmt.Sprintf("user:online:%d", userID)
	return cr.redis.Get(ctx, key).Time()
}

func (cr *ConnectionRepo) GetAllOnlineUsers(ctx context.Context) ([]int, error) {
	keys, err := cr.redis.Keys(ctx, "user:online:*").Result()
	if err != nil {
		return nil, err
	}

	userIDs := make([]int, 0, len(keys))
	for _, key := range keys {
		userIDStr := strings.TrimPrefix(key, "user:online:")
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			continue
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

func (cr *ConnectionRepo) DeleteOnlineStatus(ctx context.Context, userID int) error {
	key := fmt.Sprintf("user:online:%d", userID)
	return cr.redis.Del(ctx, key).Err()
}
