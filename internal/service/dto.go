package service

import (
	"encoding/json"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

type NewMessage struct {
	Type    domain.ChatType `json:"type"`
	ChatID  int             `json:"chat_id"`
	Content string          `json:"content"`
}

type ProduceMessage struct {
	Type domain.EventType `json:"type"`
	Data json.RawMessage  `json:"data,omitempty"`
}

// events to client
type Presence struct {
	UserID    int       `json:"user_id"`
	Presence  bool      `json:"presence"`
	Timestamp time.Time `json:"timestamp"`
}

type MessageEvent struct {
	MessageID  int       `json:"message_id"`
	FromUserID int       `json:"from_user_id"`
	ChatID     int       `json:"chat_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// добавление, удаление и изменение прав
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
	Role   domain.GroupMemberRole
	ChatID int
	UserID int
}

type PaginateMessagesDTO struct {
	UserID int
	ChatID int
	Cursor *int
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
