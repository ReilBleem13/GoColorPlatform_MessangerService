package service

import (
	"encoding/json"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

type EventType string

const (
	PrivateMessageType EventType = "PRIVATE_MESSAGE"
	GroupMessageType   EventType = "GROUP_MESSAGE"

	NewGroupMemberType  EventType = "NEW_MEMBER"
	MemberLeftGroupType EventType = "MEMBER_LEFT"

	InvitedToGroup EventType = "INVITED_TO_GROUP"
	PresenceChange EventType = "PRESENCE_CHANGE"
)

// ReceiverID может быть как group_id, так и to_user_id
// в зависимости от типа сообщения
type NewMessage struct {
	TypeMessage EventType `json:"type"`
	Content     string    `json:"content"`
	ReceiverID  int       `json:"receiver_id"`
}

type ProduceMessage struct {
	TypeMessage EventType       `json:"message_type"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// response
type UserGroup struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type OnlineUsersWithLastTimestamp struct {
	UserID     int
	Timestampt time.Time
}

// events to client
type Presence struct {
	UserID    int       `json:"user_id"`
	Presence  bool      `json:"presence"`
	Timestamp time.Time `json:"timestamp"`
}

type PrivateMessageEvent struct {
	MessageID  int       `json:"message_id"`
	FromUserID int       `json:"from_user_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type GroupMessageEvent struct {
	MessageID  int       `json:"message_id"`
	GroupID    int       `json:"group_id"`
	FromUserID int       `json:"from_user_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// добавление, удаление и изменение прав
type GroupChangeMemberStatusEvent struct {
	MessageID int `json:"message_id"`
	GroupID   int `json:"group_id"`
	UserID    int `json:"user_id"`
}

type InvitedToGroupEvent struct {
	GroupID     int `json:"group_id"`
	InvitedByID int `json:"invited_by_id"`
}

// DTOs

type UpdateGroupMemberRoleDTO struct {
	Role    domain.GroupMemberRole
	GroupID int
	UserID  int
}

type PaginatePrivateMessagesDTO struct {
	User1  int
	User2  int
	Cursor *int
}

type PaginateGroupMessagesDTO struct {
	GroupID int
	Cursor  *int
}
