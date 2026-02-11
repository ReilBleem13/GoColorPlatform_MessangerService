package service

import (
	"encoding/json"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

type MessageType string

const (
	PrivateMessageType MessageType = "PRIVATE_MESSAGE"
	GroupMessageType   MessageType = "GROUP_MESSAGE"

	NewGroupMember  MessageType = "NEW_MEMBER"
	ExitGroupMember MessageType = "EXIT_MEMBER"

	UserOnlineStatus  MessageType = "USER_ONLINE"
	UserOfflineStatus MessageType = "USER_OFFLINE"
)

type NewMessageRaw struct {
	TypeMessage MessageType     `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type PrivateMessage struct {
	ToUserID int    `json:"to_user_id"`
	Content  string `json:"content"`
}

type GroupMessage struct {
	GroupID int    `json:"group_id"`
	Content string `json:"content"`
}

type ProduceMessage struct {
	MessageID   int         `json:"message_id"`
	TypeMessage MessageType `json:"message_type"`
	FromUserID  int         `json:"from_user_id"`
	CreatedAt   time.Time   `json:"created_at"`

	Content string `json:"content,omitempty"`
	GroupID *int   `json:"group_id,omitempty"`

	Payload json.RawMessage `json:"payload,omitempty"`
}

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

// response
type UserGroup struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
