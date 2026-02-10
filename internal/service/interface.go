package service

import (
	"context"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/redis/go-redis/v9"
)

type MessageRepoIn interface {
	NewMessage(ctx context.Context, fromUserID int, in *PrivateMessage) (int, error)
	UpdateMessageStatus(ctx context.Context, messageID int, status domain.MessageStatus) error

	NewGroup(ctx context.Context, name string, authorID int) (int, error)
	DeleteGroup(ctx context.Context, userID, groupID int) error
	NewGroupMember(ctx context.Context, groupID, userID int) error
	DeleteGroupMember(ctx context.Context, groupID int, userID int) error
	GetAllGroupMembers(ctx context.Context, groupID int) ([]int, error)
	GetUserGroups(ctx context.Context, userID int) ([]UserGroup, error)
	ChangeGroupMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error
	NewGroupMessage(ctx context.Context, groupID int, fromUserID int, content string) (int, error)
	PaginateGroupMessages(ctx context.Context, groupID int, messageID *int) ([]ProduceMessage, *int, bool, error)
	PaginatePrivateMessages(ctx context.Context, userID1, userID2 int, cursor *int) ([]ProduceMessage, *int, bool, error)

	UpdateGroupMessageStatus(ctx context.Context, messageID, userID int, status domain.MessageStatus) error
}

type ConnectionRepoIn interface {
	Online(ctx context.Context, userID int) error
	IsOnline(ctx context.Context, userID int) (bool, error)
	Subscribe(ctx context.Context, userID int) *redis.PubSub
	Produce(ctx context.Context, channel string, msg *ProduceMessage) error
}

type MessageServiceIn interface {
	HandleConn(ctx context.Context, client *Client)

	NewGroup(ctx context.Context, name string, authorID int) (int, error)
	DeleteGroup(ctx context.Context, groupID, userID int) error
	NewGroupMember(ctx context.Context, groupID, userID int) error
	DeleteGroupMember(ctx context.Context, groupID, userID int) error
	GetAllGroupMembers(ctx context.Context, groupID int) ([]int, error)
	GetUserGroups(ctx context.Context, userID int) ([]UserGroup, error)
	ChangeGroupMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error

	NewGroupMessage(ctx context.Context, groupID, fromUserID int, content string) (int, error)
	PaginatePrivateMessages(ctx context.Context, in *PaginatePrivateMessagesDTO) ([]ProduceMessage, *int, bool, error)
	PaginateGroupMessages(ctx context.Context, in *PaginateGroupMessagesDTO) ([]ProduceMessage, *int, bool, error)
}
