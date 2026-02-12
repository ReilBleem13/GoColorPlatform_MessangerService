package service

import (
	"context"
	"encoding/json"
	"log/slog"
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

func (ms *MessageService) NewGroupMember(ctx context.Context, in *GroupMemberDTO) error {
	slog.Debug("User trying to add other user to group",
		"subjectID", in.SubjectID,
		"objectID", in.ObjectID,
		"groupID", in.GroupID,
	)

	messageID, err := ms.msgRepo.NewGroupMember(ctx, in.GroupID, in.ObjectID)
	if err != nil {
		slog.Error("Failed to create new group member", "error", err)
		return err
	}

	memberIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, in.GroupID)
	if err != nil {
		slog.Error("Failed to get all group members", "error", err)
		return nil
	}

	changeListOfGroupsEvent := ChangeListOfGroupsEvent{
		GroupID: in.GroupID,
	}

	changeListOfGroupsEventByte, err := json.Marshal(changeListOfGroupsEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	ms.handleProduce(ctx, in.ObjectID, &ProduceMessage{
		TypeMessage: InvitedToGroup,
		Data:        changeListOfGroupsEventByte,
	})

	newMemberEvent := GroupChangeMemberStatusEvent{
		MessageID: messageID,
		GroupID:   in.GroupID,
		UserID:    in.ObjectID,
	}

	newMemberEventByte, err := json.Marshal(newMemberEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	for _, id := range memberIDs {
		ms.handleProduce(ctx, id, &ProduceMessage{
			TypeMessage: NewMemberType,
			Data:        newMemberEventByte,
		})
	}
	return nil
}

func (ms *MessageService) DeleteGroupMember(ctx context.Context, in *GroupMemberDTO) error {
	messageID, err := ms.msgRepo.DeleteGroupMember(ctx, in.GroupID, in.ObjectID, *in.Type)
	if err != nil {
		slog.Error("Failed to delete group member", "error", err)
		return err
	}

	memberIDs, err := ms.msgRepo.GetAllGroupMembers(ctx, in.GroupID)
	if err != nil {
		slog.Error("Failed to get all group members", "error", err)
		return err
	}

	changeListOfGroupsEvent := ChangeListOfGroupsEvent{
		GroupID: in.GroupID,
	}

	changeListOfGroupsEventByte, err := json.Marshal(changeListOfGroupsEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	ms.handleProduce(ctx, in.ObjectID, &ProduceMessage{
		TypeMessage: DeletedFromGroup,
		Data:        changeListOfGroupsEventByte,
	})

	kickedMemberEvent := GroupChangeMemberStatusEvent{
		MessageID: messageID,
		GroupID:   in.GroupID,
		UserID:    in.ObjectID,
	}

	kickedMemberEventByte, err := json.Marshal(kickedMemberEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	for _, id := range memberIDs {
		if id != in.ObjectID {
			ms.handleProduce(ctx, id, &ProduceMessage{
				TypeMessage: KickedMemberType,
				Data:        kickedMemberEventByte,
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
