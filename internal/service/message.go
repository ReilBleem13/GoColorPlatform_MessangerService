package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 10 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type MessageService struct {
	heartbeatService HeartbeatServiceIn
	msgRepo          MessageRepoIn
	connRepo         ConnectionRepoIn
}

func NewMessageService(heartbeatService HeartbeatServiceIn, msgRepo MessageRepoIn, connRepo ConnectionRepoIn) MessageServiceIn {
	return &MessageService{
		msgRepo:  msgRepo,
		connRepo: connRepo,
	}
}

func (ms *MessageService) HandleConn(ctx context.Context, client *Client) {
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))

		slog.Debug("Handle heartbeat", "client_id", client.id)

		if err := ms.heartbeatService.HandleHeartbeat(ctx, client.id); err != nil {
			slog.Error("Failed t0 handle heartbeat", "user_id", client.id, "error", err)
		}

		return nil
	})

	ms.heartbeatService.HandleHeartbeat(ctx, client.id)

	pubSub := ms.connRepo.Subscribe(ctx, client.id)
	client.outboard = pubSub.Channel()

	defer func() {
		client.hub.unregister <- client
		client.conn.Close()
		pubSub.Close()
	}()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return ms.read(ctx, client)
	})

	g.Go(func() error {
		return ms.write(ctx, client)
	})

	err := g.Wait()
	if err != nil && err != context.Canceled {
		slog.Error("Error during handle Conn", "error", err)
	}
}

func (ms *MessageService) read(ctx context.Context, client *Client) error {
	client.conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var newMessage NewMessage
			if err := client.conn.ReadJSON(&newMessage); err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
					websocket.CloseNoStatusReceived) {
					slog.Error("Websoket close error", "error", err)
				}
				return context.Canceled
			}

			switch newMessage.TypeMessage {
			case PrivateMessageType:
				ms.handlePrivateMessage(ctx, client, &newMessage)

			case GroupMessageType:
				ms.handleGroupMessage(ctx, client, &newMessage)

			default:
				slog.Error("Failed to define type message",
					"user_id", client.id,
					"type", newMessage.TypeMessage,
				)
				return domain.ErrInvalidRequest
			}
		}
	}
}

func (ms *MessageService) handlePrivateMessage(ctx context.Context, client *Client, msg *NewMessage) {
	messageID, err := ms.msgRepo.NewMessage(ctx, client.id, msg)
	if err != nil {
		log.Printf("Failed to save message to DB for user %d: %v", client.id, err)
	}

	data := PrivateMessageEvent{
		MessageID:  messageID,
		FromUserID: client.id,
		Content:    msg.Content,
		CreatedAt:  time.Now(),
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return
	}

	ms.handleProduce(ctx, msg.ReceiverID, &ProduceMessage{
		TypeMessage: PrivateMessageType,
		Data:        dataByte,
	})
}

func (ms *MessageService) handleGroupMessage(ctx context.Context, client *Client, msg *NewMessage) {
	messageID, err := ms.msgRepo.NewGroupMessage(ctx, msg.ReceiverID, client.id, msg.Content)
	if err != nil {
		slog.Error("Failed to save message to DB", "user_id", client.id, "error", err)
		return
	}

	groupMembersIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, msg.ReceiverID)
	if err != nil {
		slog.Error("Failed tp get all group members", "error", err)
		return
	}

	data := GroupMessageEvent{
		MessageID:  messageID,
		GroupID:    msg.ReceiverID,
		FromUserID: client.id,
		Content:    msg.Content,
		CreatedAt:  time.Now(),
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return
	}

	produceMsg := &ProduceMessage{
		TypeMessage: GroupMessageType,
		Data:        dataByte,
	}

	for _, memberID := range groupMembersIDs {
		if memberID != client.id {
			ms.handleProduce(ctx, memberID, produceMsg)
		}
	}
}

func (ms *MessageService) handleProduce(ctx context.Context, toUserID int, msg *ProduceMessage) {
	timestamp, err := ms.connRepo.GetOnlineStatus(ctx, toUserID)
	if err != nil {
		slog.Error("Failed to get online status", "error", err, "to_user_id", toUserID)
		return
	}

	threshold := defaultInterval + 2*defaultDelta
	now := time.Now()

	if now.Sub(timestamp) > threshold {
		return
	}

	channel := fmt.Sprintf("message:%d", toUserID)

	err = ms.connRepo.Produce(ctx, channel, msg)
	if err != nil {
		slog.Error("Failed to produce message", "to_user_id", toUserID, "error", err)
	} else {
		slog.Info("Message successfully produced", "to_user_id", toUserID)
	}
}

func (ms *MessageService) write(ctx context.Context, client *Client) error {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("Failed to write ping message", "error", err)
				return err
			}
		case msg, ok := <-client.outboard:
			if !ok {
				return nil
			}

			var outboardMsg ProduceMessage
			if err := json.Unmarshal([]byte(msg.Payload), &outboardMsg); err != nil {
				slog.Error("Failed to unmarshal outboard message", "error", err)
				return err
			}

			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteJSON(&outboardMsg); err != nil {
				slog.Error("Failed to writeJSON", "error", err)
				return err
			}

			switch outboardMsg.TypeMessage {
			case PrivateMessageType:
				var v PrivateMessageEvent
				if err := json.Unmarshal(outboardMsg.Data, &v); err != nil {
					slog.Error("Failed to unmarshal", "error", err)
					continue
				}

				if err := ms.msgRepo.UpdateMessageStatus(ctx, v.MessageID, domain.StatusDelivered); err != nil {
					log.Printf("Failed to update private message status. Message %+v, status: %s, error: %v", msg, domain.StatusDelivered, err)
					continue
				}
			case GroupMessageType:
				var v GroupMessageEvent
				if err := json.Unmarshal(outboardMsg.Data, &v); err != nil {
					slog.Error("Failed to unmarshal", "error", err)
					continue
				}

				if err := ms.msgRepo.UpdateGroupMessageStatus(ctx, v.MessageID, client.id, domain.StatusDelivered); err != nil {
					log.Printf("Failed to update group message status. Message %+v, status: %s, error: %v", msg, domain.StatusDelivered, err)
					continue
				}
			}
		}
	}
}
