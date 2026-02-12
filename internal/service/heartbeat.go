package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type HeartbeatService struct {
	connRepo ConnectionRepoIn
	interval time.Duration
	delta    time.Duration
	timers   sync.Map
}

func NewHeartbeatService(connRepo ConnectionRepoIn) HeartbeatServiceIn {
	hs := &HeartbeatService{
		connRepo: connRepo,
		interval: pongWait,
		delta:    5 * time.Second,
	}

	go hs.offlineScanner(context.Background())

	return hs
}

func (hs *HeartbeatService) HandleHeartbeat(ctx context.Context, userID int) error {
	now := time.Now()

	lastActive, err := hs.connRepo.GetOnlineStatus(ctx, userID)

	wasOffline := err == redis.Nil || time.Since(lastActive) > (hs.interval-hs.delta)

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
	ticker := time.NewTicker(15 * time.Second)
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
	onlineUsers, err := hs.connRepo.GetAllOnlineUsers(ctx)
	if err != nil {
		slog.Error("Failed to get online users", "error", err)
		return
	}

	threshold := hs.interval + hs.delta*2
	now := time.Now()

	for _, userID := range onlineUsers {
		lastActive, err := hs.connRepo.GetOnlineStatus(ctx, userID)
		if err != nil {
			continue
		}

		if now.Sub(lastActive) > threshold {
			hs.connRepo.DeleteOnlineStatus(ctx, userID)

			hs.notifyPresenceChange(ctx, userID, false)

			slog.Info("User went offline", "user_id", userID)
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
		TypeMessage: PresenceChange,
		Payload:     marshalData,
	}

	for _, recipientID := range interestedUsers {
		channel := fmt.Sprintf("message:%d", recipientID)
		hs.connRepo.Produce(ctx, channel, msg)
	}
}

func (hs *HeartbeatService) getInterestedUsers(ctx context.Context, userID int) ([]int, error) {
	return nil, nil
}

func (hs *HeartbeatService) IsUserOnline(ctx context.Context, userID int) bool {
	lastActive, err := hs.connRepo.GetOnlineStatus(ctx, userID)
	if err != nil {
		return false
	}

	return time.Since(lastActive) <= (hs.interval - hs.delta)
}
