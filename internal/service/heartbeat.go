package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	defaultDelta                  = 5 * time.Second
	defaultInterval               = pongWait
	defaultOfflineScannerInterval = 15 * time.Second
)

type HeartbeatOption func(ho *HeartbeatService)

type HeartbeatService struct {
	connRepo               ConnectionRepoIn
	msgRepo                MessageRepoIn
	offlineScannerInterval time.Duration
	interval               time.Duration
	delta                  time.Duration
}

func WithInterval(interval time.Duration) HeartbeatOption {
	return func(ho *HeartbeatService) {
		ho.interval = interval
	}
}

func WithDelta(delta time.Duration) HeartbeatOption {
	return func(ho *HeartbeatService) {
		ho.delta = delta
	}
}

func WithOfflineInterval(offlineScannerInterval time.Duration) HeartbeatOption {
	return func(ho *HeartbeatService) {
		ho.offlineScannerInterval = offlineScannerInterval
	}
}

func NewHeartbeatService(ctx context.Context, connRepo ConnectionRepoIn, opts ...HeartbeatOption) HeartbeatServiceIn {
	hs := &HeartbeatService{
		connRepo: connRepo,
		interval: defaultInterval,
		delta:    defaultDelta,
	}

	for _, opt := range opts {
		opt(hs)
	}

	go hs.offlineScanner(ctx)
	return hs
}

func (hs *HeartbeatService) HandleHeartbeat(ctx context.Context, userID int) error {
	now := time.Now()

	lastActive, err := hs.connRepo.GetOnlineStatus(ctx, userID)

	wasOffline := err == redis.Nil || time.Since(lastActive) > (hs.interval+hs.delta)

	hs.connRepo.UpdateOnlineStatus(ctx, &Presence{
		UserID:    userID,
		Presence:  true,
		Timestamp: now,
	})

	if wasOffline {
		hs.notifyPresenceChange(ctx, userID, true)
	}
	return nil
}

func (hs *HeartbeatService) offlineScanner(ctx context.Context) {
	ticker := time.NewTicker(hs.offlineScannerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hs.checkOfflineUsers(ctx)
		}
	}
}

func (hs *HeartbeatService) checkOfflineUsers(ctx context.Context) {
	onlineUsersWithTimestamp, err := hs.connRepo.GetAllOnlineUsers(ctx)
	if err != nil {
		slog.Error("Failed to get online users", "error", err)
		return
	}

	now := time.Now()
	threshold := hs.interval + 2*hs.delta

	for _, user := range onlineUsersWithTimestamp {
		if now.Sub(user.Timestampt) > threshold {
			hs.connRepo.DeleteOnlineStatus(ctx, user.UserID)
			hs.notifyPresenceChange(ctx, user.UserID, false)
			slog.Debug("User went offline", "user_id", user.UserID)
		}
	}
}

func (hs *HeartbeatService) notifyPresenceChange(ctx context.Context, userID int, isOnline bool) {
	interestedUsers, err := hs.getInterestedUsers(ctx, userID)
	if err != nil {
		slog.Error("Failed to get interested users")
	}

	marshalData, err := json.Marshal(Presence{
		UserID:    userID,
		Presence:  isOnline,
		Timestamp: time.Now(),
	})

	msg := &ProduceMessage{
		Type: domain.PresenceChangeType,
		Data: marshalData,
	}

	for _, recipientID := range interestedUsers {
		channel := fmt.Sprintf("message:%d", recipientID)
		hs.connRepo.Produce(ctx, channel, msg)
	}
}

func (hs *HeartbeatService) getInterestedUsers(ctx context.Context, userID int) ([]int, error) {
	ids, err := hs.msgRepo.GetUserContacts(ctx, userID)
	if err != nil {
		slog.Error("Failed to get all user contacts", "error", err)
		return nil, err
	}

	result := []int{}
	now := time.Now()

	for _, id := range ids {
		timestamp, err := hs.connRepo.GetOnlineStatus(ctx, id)
		if err != nil {
			slog.Error("Failed to get online status", "error", err)
			continue
		}

		threshold := hs.interval + 2*hs.delta
		if now.Sub(timestamp) < threshold {
			result = append(result, id)
		}
	}
	return result, nil
}

func (hs *HeartbeatService) IsUserOnline(ctx context.Context, userID int) bool {
	lastActive, err := hs.connRepo.GetOnlineStatus(ctx, userID)
	if err != nil {
		return false
	}
	return time.Since(lastActive) <= (hs.interval + 2*hs.delta)
}
