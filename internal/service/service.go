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
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type MessageService struct {
	msgRepo  MessageRepoIn
	connRepo ConnectionRepoIn
}

func NewMessageService(msgRepo MessageRepoIn, connRepo ConnectionRepoIn) MessageServiceIn {
	return &MessageService{
		msgRepo:  msgRepo,
		connRepo: connRepo,
	}
}

func (ms *MessageService) HandleConn(ctx context.Context, client *Client) {
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		err := ms.connRepo.Online(ctx, client.id)
		if err != nil {
			return err
		}
		return nil
	})

	if err := ms.connRepo.Online(ctx, client.id); err != nil {
		log.Printf("Failed to set user %d online: %v", client.id, err)
	}

	ms.notifyStatusChange(ctx, client.id, UserOnlineStatus)

	pubSub := ms.connRepo.Subscribe(ctx, client.id)
	client.outboard = pubSub.Channel()

	defer func() {
		if err := ms.connRepo.Offline(ctx, client.id); err != nil {
			log.Printf("Failed to set user %d offline: %v", client.id, err)
		}
		ms.notifyStatusChange(context.Background(), client.id, UserOfflineStatus)

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
			var wrapper NewMessageRaw
			if err := client.conn.ReadJSON(&wrapper); err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
					websocket.CloseNoStatusReceived) {
					slog.Error("Websoket close error", "error", err)
				}
				return context.Canceled
			}

			switch wrapper.TypeMessage {
			case PrivateMessageType:
				var privateMsg PrivateMessage
				if err := json.Unmarshal(wrapper.Data, &privateMsg); err != nil {
					slog.Error("Failed to unmarshal rawMessage for PrivateMessage",
						"user_id", client.id,
						"error", err,
					)
					return domain.ErrInvalidRequest
				}
				ms.handlePrivateMessage(ctx, client, &privateMsg)

			case GroupMessageType:
				var groupMsg GroupMessage
				if err := json.Unmarshal(wrapper.Data, &groupMsg); err != nil {
					slog.Error("Failed to unmarshal rawMessage for GroupMessage",
						"user_id", client.id,
						"error", err,
					)
					return domain.ErrInvalidRequest
				}
				ms.handleGroupMessage(ctx, client, &groupMsg)

			default:
				slog.Error("Failed to define type message",
					"user_id", client.id,
					"type", wrapper.TypeMessage,
				)
				return domain.ErrInvalidRequest
			}
		}
	}
}

func (ms *MessageService) handlePrivateMessage(ctx context.Context, client *Client, msg *PrivateMessage) {
	messageID, err := ms.msgRepo.NewMessage(ctx, client.id, msg)
	if err != nil {
		log.Printf("Failed to save message to DB for user %d: %v", client.id, err)
	}

	ms.handleProduce(ctx, msg.ToUserID, &ProduceMessage{
		MessageID:   messageID,
		TypeMessage: PrivateMessageType,
		FromUserID:  client.id,
		CreatedAt:   time.Now(),
		Content:     msg.Content,
	})
}

func (ms *MessageService) handleGroupMessage(ctx context.Context, client *Client, msg *GroupMessage) {
	messageID, err := ms.msgRepo.NewGroupMessage(ctx, msg.GroupID, client.id, msg.Content)
	if err != nil {
		log.Printf("Failed to save message to DB for user %d: %v", client.id, err)
		return
	}

	groupMembersIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, msg.GroupID)
	if err != nil {
		log.Printf("Failed to get all group members: %v", err)
		return
	}

	produceMsg := &ProduceMessage{
		MessageID:   messageID,
		TypeMessage: GroupMessageType,
		FromUserID:  client.id,
		CreatedAt:   time.Now(),
		Content:     msg.Content,
		GroupID:     &msg.GroupID,
	}

	for _, memberID := range groupMembersIDs {
		if memberID != client.id {
			ms.handleProduce(ctx, memberID, produceMsg)
		}
	}
}

func (ms *MessageService) handleProduce(ctx context.Context, toUserID int, msg *ProduceMessage) {
	isOnline, err := ms.connRepo.IsOnline(ctx, toUserID)
	if err != nil {
		log.Printf("Failed to check online status for user %d: %v", toUserID, err)
		return
	}

	if !isOnline {
		log.Printf("User %d is not online now", toUserID)
		return
	}

	channel := fmt.Sprintf("message:%d", toUserID)

	err = ms.connRepo.Produce(ctx, channel, msg)
	if err != nil {
		log.Printf("Failed to produce message %d: %v", msg.MessageID, err)
	} else {
		log.Printf("Message %d successfully produced", msg.MessageID)
	}
}

func (ms *MessageService) notifyStatusChange(ctx context.Context, userID int, status MessageType) {
	contacts, err := ms.msgRepo.GetUserContacts(ctx, userID)
	if err != nil {
		log.Printf("Failed to get contacts for user %d: %v", userID, err)
		return
	}

	produceMsg := &ProduceMessage{
		MessageID:   0,
		TypeMessage: status,
		FromUserID:  userID,
		CreatedAt:   time.Now(),
	}

	for _, contact := range contacts {
		if contact != userID {
			ms.handleProduce(ctx, contact, produceMsg)
		}
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
				log.Printf("Failed to write ping message: %v", err)
				return err
			}
		case msg, ok := <-client.outboard:
			if !ok {
				return nil
			}

			var outboardMsg ProduceMessage
			if err := json.Unmarshal([]byte(msg.Payload), &outboardMsg); err != nil {
				log.Printf("Failed to unmarshal outboard message: %v", err)
				return err
			}

			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteJSON(&outboardMsg); err != nil {
				log.Printf("Failed to writeJSON. Message %+v, error: %v", msg, err)
				return err
			}

			switch outboardMsg.TypeMessage {
			case PrivateMessageType:
				if err := ms.msgRepo.UpdateMessageStatus(ctx, outboardMsg.MessageID, domain.StatusDelivered); err != nil {
					log.Printf("Failed to update private message status. Message %+v, status: %s, error: %v", msg, domain.StatusDelivered, err)
					continue
				}
			case GroupMessageType:
				if err := ms.msgRepo.UpdateGroupMessageStatus(ctx, outboardMsg.MessageID, client.id, domain.StatusDelivered); err != nil {
					log.Printf("Failed to update group message status. Message %+v, status: %s, error: %v", msg, domain.StatusDelivered, err)
					continue
				}
			}
		}
	}
}

// GROUPS
func (ms *MessageService) NewGroup(ctx context.Context, name string, authorID int) (int, error) {
	groupID, err := ms.msgRepo.NewGroup(ctx, name, authorID)
	if err != nil {
		slog.Error("Failed to create new group", "error", err)
		return 0, err
	}
	return groupID, nil
}

func (ms *MessageService) DeleteGroup(ctx context.Context, groupID, userID int) error {
	if err := ms.msgRepo.DeleteGroup(ctx, userID, groupID); err != nil {
		slog.Error("Failed to detele group", "error", err)
		return err
	}
	return nil
}

func (ms *MessageService) NewGroupMember(ctx context.Context, groupID, userID int) error {
	messageID, err := ms.msgRepo.NewGroupMember(ctx, groupID, userID)
	if err != nil {
		slog.Error("Failed to create new group member", "error", err)
		return err
	}

	memberIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, groupID)
	if err != nil {
		slog.Error("Failed to get all group members", "error", err)
		return err
	}

	for _, id := range memberIDs {
		if id != userID {
			ms.handleProduce(ctx, id, &ProduceMessage{
				MessageID:   messageID,
				TypeMessage: NewGroupMember,
				FromUserID:  userID,
				CreatedAt:   time.Now(),
				GroupID:     &groupID,
			})
		}
	}
	return nil
}

func (ms *MessageService) DeleteGroupMember(ctx context.Context, groupID, userID int) error {
	messageID, err := ms.msgRepo.DeleteGroupMember(ctx, groupID, userID)
	if err != nil {
		slog.Error("Failed to delete group member", "error", err)
		return err
	}

	memberIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, groupID)
	if err != nil {
		slog.Error("Failed to get all group members", "error", err)
		return err
	}

	for _, id := range memberIDs {
		if id != userID {
			ms.handleProduce(ctx, id, &ProduceMessage{
				MessageID:   messageID,
				TypeMessage: ExitGroupMember,
				FromUserID:  userID,
				CreatedAt:   time.Now(),
				GroupID:     &groupID,
			})
		}
	}
	return nil
}

func (ms *MessageService) GetAllGroupMembers(ctx context.Context, groupID int) ([]int, error) {
	members, err := ms.msgRepo.GetAllGroupMembers(ctx, groupID)
	if err != nil {
		slog.Error("Failed to get all group members", "error", err)
		return nil, err
	}
	return members, nil
}

func (ms *MessageService) GetUserGroups(ctx context.Context, userID int) ([]UserGroup, error) {
	groups, err := ms.msgRepo.GetUserGroups(ctx, userID)
	if err != nil {
		slog.Error("Failed to get user groups", "error", err)
		return nil, err
	}

	result := make([]UserGroup, len(groups))
	for i, g := range groups {
		result[i].ID = g.ID
		result[i].Name = g.Name
	}
	return result, nil
}

func (ms *MessageService) ChangeGroupMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error {
	if err := ms.msgRepo.ChangeGroupMemberRole(ctx, in); err != nil {
		slog.Error("Failed to change group member role", "error", err)
		return err
	}
	return nil
}

func (ms *MessageService) NewGroupMessage(ctx context.Context, groupID, fromUserID int, content string) (int, error) {
	groupMessageID, err := ms.msgRepo.NewGroupMessage(ctx, groupID, fromUserID, content)
	if err != nil {
		slog.Error("Failed to create new group message", "error", err)
		return 0, err
	}
	return groupMessageID, nil
}

func (ms *MessageService) PaginatePrivateMessages(ctx context.Context, in *PaginatePrivateMessagesDTO) ([]ProduceMessage, *int, bool, error) {
	messages, newCursor, hasMore, err := ms.msgRepo.PaginatePrivateMessages(ctx, in.User1, in.User2, in.Cursor)
	if err != nil {
		slog.Error("Failed to paginate private messages", "error", err)
		return nil, nil, false, err
	}

	return messages, newCursor, hasMore, nil
}

func (ms *MessageService) PaginateGroupMessages(ctx context.Context, in *PaginateGroupMessagesDTO) ([]ProduceMessage, *int, bool, error) {
	messages, newCursor, hasMore, err := ms.msgRepo.PaginateGroupMessages(ctx, in.GroupID, in.Cursor)
	if err != nil {
		slog.Error("Failed to paginate group messages", "error", err)
		return nil, nil, false, err
	}

	return messages, newCursor, hasMore, nil
}
