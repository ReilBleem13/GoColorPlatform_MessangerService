package domain

type Message struct {
	ID         int    `json:"id"`
	FromUserID int    `json:"from_user_id"`
	ToUserID   int    `json:"to_user_id"`
	Content    string `json:"content"`
}

type (
	MessageStatus   string
	GroupMemberRole string
)

const (
	StatusSent      MessageStatus = "SENT"
	StatusDelivered MessageStatus = "DELIVERED"
	StatusRead      MessageStatus = "READ"

	MemberRole GroupMemberRole = "MEMBER"
	AdminRole  GroupMemberRole = "ADMIN"
)
