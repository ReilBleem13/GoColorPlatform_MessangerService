package service

import (
	"context"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/redis/go-redis/v9"
)

type MessageRepoIn interface {
	NewMessage(ctx context.Context, in *domain.Message) (int, error)
	PaginateMessages(ctx context.Context, chatID int, cursor *int) ([]domain.Message, *int, bool, error)
	GetAllUndeliveredMessages(ctx context.Context, userID int) ([]domain.Message, error)
	UpdateMessageStatus(ctx context.Context, messageID, userID int, status domain.MessageStatus) error

	NewGroupChat(ctx context.Context, name string, authorID int) (int, error)
	DeleteGroupChat(ctx context.Context, chatID, authorID int) error

	NewGroupChatMember(ctx context.Context, chatID, userID int) (int, error)
	DeleteGroupMember(ctx context.Context, chatID, userID int, typeDelete domain.EventType) (int, error)
	GetAllChatMembers(ctx context.Context, chatID int) ([]int, error)
	GetGroupChatMemberRole(ctx context.Context, userID, chatID int) (domain.GroupMemberRole, error)
	ChangeGroupChatMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error

	GetUserChats(ctx context.Context, userID int) ([]domain.UserChat, error)
	GetUserContacts(ctx context.Context, userID int) ([]int, error)
}

type ConnectionRepoIn interface {
	Subscribe(ctx context.Context, userID int) *redis.PubSub
	Produce(ctx context.Context, channel string, msg *ProduceMessage) error

	UpdateOnlineStatus(ctx context.Context, in *Presence) error
	GetOnlineStatus(ctx context.Context, userID int) (time.Time, error)

	GetAllOnlineUsers(ctx context.Context) ([]OnlineUsersWithLastTimestamp, error)
	DeleteOnlineStatus(ctx context.Context, userID int) error
}

type MessageServiceIn interface {
	HandleConn(ctx context.Context, client *Client)

	NewGroupChat(ctx context.Context, name string, authorID int) (int, error)
	DeleteGroupChat(ctx context.Context, groupID, userID int) error
	PaginateMessages(ctx context.Context, in *PaginateMessagesDTO) ([]domain.Message, *int, bool, error)

	GetUserChats(ctx context.Context, userID int) ([]domain.UserChat, error)
	NewGroupMember(ctx context.Context, in *GroupMemberDTO) error
	DeleteGroupMember(ctx context.Context, in *GroupMemberDTO) error
	GetAllGroupChatMembers(ctx context.Context, chatID int) ([]int, error)
	ChangeGroupMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error
}

type HeartbeatServiceIn interface {
	HandleHeartbeat(ctx context.Context, userID int) error
	IsUserOnline(ctx context.Context, userID int) bool
}
