package service

import (
	"context"
	"log/slog"
	"time"
)

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
