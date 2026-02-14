package domain

import "time"

type Message struct {
	ID          int       `json:"id" db:"id"`
	ChatID      int       `json:"chat_id" db:"chat_id"`
	FromUserID  int       `json:"from_user_id" db:"from_user_id"`
	MessageType EventType `json:"message_type" db:"message_type"`
	Content     string    `json:"content,omitempty" db:"content"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type UserChat struct {
	ID        int       `json:"id" db:"id"`
	Type      ChatType  `json:"type" db:"type"`
	Name      *string   `json:"name,omitempty" db:"name"`
	AuthorID  *int      `json:"author_id,omitempty" db:"author_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ChatMember struct {
	ID       int    `json:"id" db:"id"`
	Nickname string `json:"nickname" db:"nickname"`
	IsOnline bool   `json:"is_online"`
}

type (
	ChatType string

	MessageStatus string

	GroupMemberRole string

	EventType string
)

const (
	Private ChatType = "PRIVATE"
	Group   ChatType = "GROUP"

	StatusSent      MessageStatus = "SENT"
	StatusDelivered MessageStatus = "DELIVERED"
	StatusRead      MessageStatus = "READ"

	MemberRole GroupMemberRole = "MEMBER"
	AdminRole  GroupMemberRole = "ADMIN"

	// events
	SendMesageType       EventType = "send_message"
	NewMessageType       EventType = "new_message"
	EditMessageType      EventType = "edit_message"
	DeleteMessageType    EventType = "delete_message"
	MessageConfirmedType EventType = "message_confirmed"
	MessageDeliveredType EventType = "message_delivered"
	MessageReadType      EventType = "message_read"
	NewChatType          EventType = "new_chat"

	NewMemberType    EventType = "new_member"
	LeftMemberType   EventType = "left_member"
	KickedMemberType EventType = "kicked_member"

	InvitedToGroupChatType   EventType = "INVITED_TO_CHAT"
	DeletedFromGroupChatType EventType = "DELETED_FROM_CHAT"

	PresenceChangeType EventType = "PRESENCE_CHANGE"
)
