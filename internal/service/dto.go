package service

import (
	"encoding/json"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

// Request from client
type SendMessageRequest struct {
	Type          domain.EventType `json:"type"`
	TempMessageID string           `json:"temp_message_id"`
	Content       string           `json:"content"`

	// If chat already exists
	ChatID *int `json:"chat_id,omitempty"`

	// If chat is new
	TempChatID *string `json:"temp_chat_id,omitempty"`
	ToUserID   *int    `json:"to_user_id,omitempty"`

	// if need to edit or delete message
	MessageID *int `json:"message_id,omitempty"`

	ClientSendAt time.Time `json:"client_send_at,omitempty"`
}

type SendMarkAsReadRequest struct {
	Type   domain.EventType `json:"type"`
	ChatID int              `json:"chat_id"`
	UpToID int              `json:"up_to_id"`
}

type SendMarkAsDeliveredRequest struct {
	Type      domain.EventType `json:"type"`
	ChatID    int              `json:"chat_id"`
	MessageID int              `json:"message_id"`
}

// Events for clients
type ProduceMessage struct {
	Type domain.EventType `json:"type"`
	Data json.RawMessage  `json:"data,omitempty"`
}

type MessageConfirmedEvent struct {
	TempMessageID string    `json:"temp_message_id"`
	MessageID     int       `json:"message_id"`
	ChatID        int       `json:"chat_id"`
	TempChatID    *string   `json:"temp_chat_id,omitempty"`
	CreatedChat   bool      `json:"created_chat,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

type PresenceEvent struct {
	UserID    int       `json:"user_id"`
	Presence  bool      `json:"presence"`
	Timestamp time.Time `json:"timestamp"`
}

type ReadMessageEvent struct {
	ChatID int       `json:"chat_id"`
	UserID int       `json:"user_id"`
	UpToID int       `json:"up_to_id"`
	ReadAt time.Time `json:"read_at"`
}

type DeliveredMessageEvent struct {
	ChatID    int `json:"chat_id"`
	MessageID int `json:"message_id"`
}

type NewMessageEvent struct {
	ChatID     int       `json:"chat_id"`
	MessageID  int       `json:"message_id"`
	FromUserID int       `json:"from_user_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type EditMessageEvent struct {
	ChatID     int       `json:"chat_id"`
	MessageID  int       `json:"message_id"`
	NewContent string    `json:"new_content"`
	EditedAt   time.Time `json:"edited_at"`
}

type DeleteMessageEvent struct {
	ChatID    int `json:"chat_id"`
	MessageID int `json:"message_id"`
}

type MessageReadEvent struct {
	Type   string `json:"type"`
	ChatID int    `json:"chat_id"`
	UserID int    `json:"user_id"`
	UpToID int    `json:"up_to_id"`
	ReadAt string `json:"read_at"`
}

type NewChatEvent struct {
	ChatID     int             `json:"chat_id"`
	Type       domain.ChatType `json:"type"`
	WithUserID int             `json:"with_user_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

type GroupChangeMemberStatusEvent struct {
	MessageID int `json:"message_id"`
	GroupID   int `json:"group_id"`
	UserID    int `json:"user_id"`
}

type ChangeListOfGroupsEvent struct {
	GroupID int `json:"group_id"`
}

// DTOs
type GroupMemberDTO struct {
	GroupID   int
	SubjectID int
	ObjectID  int
	Type      *domain.EventType
}

type UpdateGroupMemberRoleDTO struct {
	Role      domain.GroupMemberRole
	SubjectID int
	ObjectID  int
	ChatID    int
}

type PaginateMessagesDTO struct {
	UserID int
	ChatID int
	Cursor *int
}

// Response
type OnlineUsersWithLastTimestamp struct {
	UserID     int
	Timestampt time.Time
}
