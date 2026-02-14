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
			var rawMessage json.RawMessage
			if err := client.conn.ReadJSON(&rawMessage); err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
					websocket.CloseNoStatusReceived) {
					slog.Error("Websoket close error", "error", err)
				}
				return context.Canceled
			}

			var typeCheck struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(rawMessage, &typeCheck); err != nil {
				slog.Error("Failed to unmarshal message type", "error", err)
				continue
			}

			switch typeCheck.Type {
			case string(domain.SendMesageType):
				var msg SendMessageRequest
				if err := json.Unmarshal(rawMessage, &msg); err != nil {
					slog.Error("Failed to unmarshal SendMessageRequest", "error", err)
					continue
				}
				ms.mapSendMessageRequest(ctx, client, &msg)

			case string(domain.MessageReadType):
				var msg SendMarkAsReadRequest
				if err := json.Unmarshal(rawMessage, &msg); err != nil {
					slog.Error("Failed to unmarshal SendMarkAsReadRequest", "error", err)
					continue
				}
			case string(domain.MessageDeliveredType):
				var msg SendMarkAsDelivered
				if err := json.Unmarshal(rawMessage, &msg); err != nil {
					slog.Error("Failed to unmarshal SendMarkAsDelivered", "error", err)
					continue
				}
			default:
				slog.Warn("Unknown message type", "type", typeCheck.Type)
			}
		}
	}
}

func (ms *MessageService) mapSendMessageRequest(ctx context.Context, client *Client, msgToSend *SendMessageRequest) {
	switch msgToSend.Type {
	case domain.SendMesageType:
		ms.handleSendMessage(ctx, client, msgToSend)
	case domain.EditMessageType:
		ms.handleEditMessage(ctx, client, msgToSend)
	case domain.DeleteMessageType:
		ms.handleDeleteMessage(ctx, client, msgToSend)
	}
}

func (ms *MessageService) handleSendMessage(ctx context.Context, client *Client, msgToSend *SendMessageRequest) {
	var (
		chatID    int
		isNewChat bool
		err       error
	)

	now := time.Now()

	if msgToSend.TempChatID != nil && msgToSend.ToUserID != nil {
		chatID, isNewChat, err = ms.msgRepo.GetOrCreatePrivateChat(ctx, client.id, *msgToSend.ToUserID)
		if err != nil {
			slog.Error("Failed to get or create private chat",
				"error", err,
				"client_id", client.id,
				"to_user_id", *msgToSend.ToUserID,
			)
			return
		}

		// send new chat event to recipient
		newChatEvent := NewChatEvent{
			Type:       domain.Private,
			ChatID:     chatID,
			WithUserID: client.id,
			CreatedAt:  now,
		}

		newChatEventByte, err := json.Marshal(&newChatEvent)
		if err != nil {
			slog.Error("Failed to marshal new chat event", "error", err)
			return
		}

		ms.handleProduce(ctx, *msgToSend.ToUserID, &ProduceMessage{
			Type: domain.NewChatType,
			Data: newChatEventByte,
		})
	} else {
		chatID = *msgToSend.ChatID
	}

	messageID, err := ms.msgRepo.NewMessage(ctx, &domain.Message{
		MessageType: domain.NewMessageType,
		ChatID:      chatID,
		FromUserID:  client.id,
		Content:     msgToSend.Content,
	})
	if err != nil {
		slog.Error("Failed to save message to DB",
			"error", err,
			"client_id", client.id,
		)
		return
	}

	// send confirmed event to sendler
	msgConfirmedEvent := MessageConfirmedEvent{
		TempMessageID: msgToSend.TempMessageID,
		MessageID:     messageID,
		TempChatID:    msgToSend.TempChatID,
		ChatID:        chatID,
		CreatedChat:   isNewChat,
		CreatedAt:     now,
	}

	msgConfirmedEventByte, err := json.Marshal(&msgConfirmedEvent)
	if err != nil {
		slog.Error("Failed to marshal msg confimed event", "error", err)
		return
	}

	ms.handleProduce(ctx, client.id, &ProduceMessage{
		Type: domain.MessageConfirmedType,
		Data: msgConfirmedEventByte,
	})

	// send new message event to recepient
	newMessageEvent := NewMessageEvent{
		ChatID:     chatID,
		MessageID:  messageID,
		FromUserID: client.id,
		Content:    msgToSend.Content,
		CreatedAt:  now,
	}

	newMessageEventByte, err := json.Marshal(&newMessageEvent)
	if err != nil {
		slog.Error("Failed to marshal new message event", "error", err)
		return
	}

	chatMembers, err := ms.msgRepo.GetAllChatMembers(ctx, chatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	for _, member := range chatMembers {
		if member.ID != client.id {
			ms.handleProduce(ctx, member.ID, &ProduceMessage{
				Type: domain.NewMessageType,
				Data: newMessageEventByte,
			})
		}
	}
	slog.Debug("Message successfully provided", "message_id", messageID, "client_id", client)
}

func (ms *MessageService) handleEditMessage(ctx context.Context, client *Client, msgToSend *SendMessageRequest) {
	if msgToSend.MessageID == nil || msgToSend.ChatID == nil {
		slog.Error("Failed to handle edit message, messageID/chatID is nil")
		return
	}

	if err := ms.msgRepo.EditMessage(ctx, *msgToSend.MessageID, msgToSend.Content); err != nil {
		slog.Error("Failed tp edit message", "error", err)
		return
	}

	now := time.Now()

	// send confirmed event to sendler
	msgConfirmedEvent := MessageConfirmedEvent{
		TempMessageID: msgToSend.TempMessageID,
		MessageID:     *msgToSend.MessageID,
		ChatID:        *msgToSend.ChatID,
		CreatedChat:   false,
		CreatedAt:     now,
	}

	msgConfirmedEventByte, err := json.Marshal(&msgConfirmedEvent)
	if err != nil {
		slog.Error("Failed to marshal msg confimed event", "error", err)
		return
	}

	ms.handleProduce(ctx, client.id, &ProduceMessage{
		Type: domain.MessageConfirmedType,
		Data: msgConfirmedEventByte,
	})

	// send edit message event to other users
	editMessageEvent := EditMessageEvent{
		ChatID:     *msgToSend.ChatID,
		MessageID:  *msgToSend.MessageID,
		NewContent: msgToSend.Content,
		EditedAt:   now,
	}

	editMessageEventByte, err := json.Marshal(&editMessageEvent)
	if err != nil {
		slog.Error("Failed to marshal edit message event", "error", err)
		return
	}

	chatMembers, err := ms.msgRepo.GetAllChatMembers(ctx, *msgToSend.ChatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	for _, member := range chatMembers {
		if member.ID != client.id {
			ms.handleProduce(ctx, member.ID, &ProduceMessage{
				Type: domain.EditMessageType,
				Data: editMessageEventByte,
			})
		}
	}
	slog.Debug("Message successfully provided", "message_id", msgToSend.MessageID, "client_id", client)
}

func (ms *MessageService) handleDeleteMessage(ctx context.Context, client *Client, msgToSend *SendMessageRequest) {
	if msgToSend.MessageID == nil || msgToSend.ChatID == nil {
		slog.Error("Failed to handle delete message, messageID/chatID is nil")
		return
	}

	authorID, err := ms.msgRepo.GetMessageAuthorID(ctx, *msgToSend.MessageID)
	if err != nil {
		slog.Error("Failed to get message author id", "error", err)
		return
	}

	if authorID != client.id {
		role, err := ms.msgRepo.GetGroupChatMemberRole(ctx, client.id, *msgToSend.ChatID)
		if err != nil {
			slog.Error("Failed to get role", "error", err)
			return
		}

		if role != domain.AdminRole {
			slog.Error("Failed to delete message, not enough right")
			return
		}
	}

	now := time.Now()

	// send confirmed event to sendler
	msgConfirmedEvent := MessageConfirmedEvent{
		TempMessageID: msgToSend.TempMessageID,
		MessageID:     *msgToSend.MessageID,
		ChatID:        *msgToSend.ChatID,
		CreatedChat:   false,
		CreatedAt:     now,
	}

	msgConfirmedEventByte, err := json.Marshal(&msgConfirmedEvent)
	if err != nil {
		slog.Error("Failed to marshal msg confimed event", "error", err)
		return
	}

	ms.handleProduce(ctx, client.id, &ProduceMessage{
		Type: domain.MessageConfirmedType,
		Data: msgConfirmedEventByte,
	})

	// send delete event to other users
	deleteMessageEvent := DeleteMessageEvent{
		ChatID:    *msgToSend.ChatID,
		MessageID: *msgToSend.MessageID,
	}

	deleteMessageEventByte, err := json.Marshal(&deleteMessageEvent)
	if err != nil {
		slog.Error("Failed to marshal delete message event", "error", err)
		return
	}

	chatMembers, err := ms.msgRepo.GetAllChatMembers(ctx, *msgToSend.ChatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	for _, member := range chatMembers {
		if member.ID != client.id {
			ms.handleProduce(ctx, member.ID, &ProduceMessage{
				Type: domain.DeleteMessageType,
				Data: deleteMessageEventByte,
			})
		}
	}
	slog.Debug("Message successfully provided", "message_id", msgToSend.MessageID, "client_id", client)
}

func (ms *MessageService) handleSendAsDelivered(ctx context.Context, client *Client, msgToSend *SendMarkAsDelivered) {
	if err := ms.msgRepo.SetDeliveredAtStatus(ctx, msgToSend.MessageID, client.id); err != nil {
		slog.Error("Failed to set delivered at status",
			"message_id", msgToSend.MessageID,
			"client_id", client.id,
			"error", err,
		)
		return
	}

	deliveredEvent := DeliveredMessageEvent{
		ChatID:    msgToSend.ChatID,
		MessageID: msgToSend.MessageID,
	}

	deliveredEventByte, err := json.Marshal(&deliveredEvent)
	if err != nil {
		slog.Error("Failed to marshal new message event", "error", err)
		return
	}

	chatMembers, err := ms.msgRepo.GetAllChatMembers(ctx, msgToSend.ChatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	// It also sends a message to the client so that he receives a confirmation of his delivery
	for _, member := range chatMembers {
		ms.handleProduce(ctx, member.ID, &ProduceMessage{
			Type: domain.MessageDeliveredType,
			Data: deliveredEventByte,
		})
	}
}

func (ms *MessageService) handleSendMarkAsRead(ctx context.Context, client *Client, msgToSend *SendMarkAsReadRequest) {
	err := ms.msgRepo.SetReadAtStatus(ctx, msgToSend.UpToID, msgToSend.ChatID, client.id)
	if err != nil {
		slog.Error("Failed to set read at status",
			"chat_id", msgToSend.ChatID,
			"client_at", client.id,
			"error", err,
		)
		return
	}

	readMessageEvent := ReadMessageEvent{
		ChatID: msgToSend.ChatID,
		UserID: client.id,
		UpToID: msgToSend.UpToID,
		ReadAt: time.Now(),
	}

	readMessageEventByte, err := json.Marshal(&readMessageEvent)
	if err != nil {
		slog.Error("Failed to marshal read message event", "error", err)
		return
	}

	chatMembers, err := ms.msgRepo.GetAllChatMembers(ctx, msgToSend.ChatID)
	if err != nil {
		slog.Error("Failed to get all chat members", "error", err)
		return
	}

	// It also sends a message to the client so that he receives a confirmation of his delivery
	for _, member := range chatMembers {
		ms.handleProduce(ctx, member.ID, &ProduceMessage{
			Type: domain.MessageReadType,
			Data: readMessageEventByte,
		})
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
