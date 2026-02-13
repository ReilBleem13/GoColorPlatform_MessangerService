package service

import (
	"context"
	"encoding/json"
	"fmt"
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

			ms.handleSendMessage(ctx, client, &newMessage)
		}
	}
}

func (ms *MessageService) handleSendMessage(ctx context.Context, client *Client, msgToSend *NewMessage) {
	messageID, err := ms.msgRepo.NewMessage(ctx, &domain.Message{
		ChatID:      msgToSend.ChatID,
		FromUserID:  client.id,
		MessageType: domain.MessageType,
		Content:     &msgToSend.Content,
	})
	if err != nil {
		slog.Error("Failed to save message to DB",
			"error", err,
			"client_id", client.id,
		)
	}

	data := MessageEvent{
		MessageID:  messageID,
		FromUserID: client.id,
		ChatID:     msgToSend.ChatID,
		Content:    msgToSend.Content,
		CreatedAt:  time.Now(),
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return
	}

	chatMemberIDs, err := ms.msgRepo.GetAllChatMembers(ctx, msgToSend.ChatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	for _, memberID := range chatMemberIDs {
		if memberID != client.id {
			ms.handleProduce(ctx, memberID, &ProduceMessage{
				Type: domain.MessageType,
				Data: dataByte,
			})
		}
	}
}

func (ms *MessageService) handleProduce(ctx context.Context, toUserID int, msgToSend *ProduceMessage) {
	if !ms.heartbeatService.IsUserOnline(ctx, toUserID) {
		slog.Debug("User is not online", "user_id", toUserID)
		return
	}

	channel := fmt.Sprintf("message:%d", toUserID)

	err := ms.connRepo.Produce(ctx, channel, msgToSend)
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

			switch outboardMsg.Type {
			case domain.MessageType:
				var v MessageEvent
				if err := json.Unmarshal(outboardMsg.Data, &v); err != nil {
					slog.Error("Failed to unmarshal message event", "error", err)
					continue
				}

				if err := ms.msgRepo.UpdateMessageStatus(ctx, v.MessageID, client.id, domain.StatusDelivered); err != nil {
					slog.Error("Failed to update message event status",
						"message_id", v.MessageID,
						"error", err,
					)
					continue
				}
			case domain.NewMemberType, domain.KickedMemberType, domain.LeftMemberType:
				var v GroupChangeMemberStatusEvent
				if err := json.Unmarshal(outboardMsg.Data, &v); err != nil {
					slog.Error("Failed to unmarshal group change member event", "error", err)
					continue
				}

				if err := ms.msgRepo.UpdateMessageStatus(ctx, v.MessageID, client.id, domain.StatusDelivered); err != nil {
					slog.Error("Failed to update group change member status event",
						"message_id", v.MessageID,
						"error", err,
					)
					continue
				}
			case domain.InvitedToGroupChatType, domain.DeletedFromGroupChatType:
				slog.Debug("Group list change event sent",
					"type", outboardMsg.Type,
					"client_id", client.id,
				)
			case domain.PresenceChangeType:
				slog.Debug("Presence change event sent", "client_id", client.id)
			default:
				slog.Warn("Unknown message type received", "type", outboardMsg.Type, "client_id", client.id)
			}
		}
	}
}
