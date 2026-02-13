package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

// GROUPS
func (ms *MessageService) NewGroupChat(ctx context.Context, name string, authorID int) (int, error) {
	groupID, err := ms.msgRepo.NewGroupChat(ctx, name, authorID)
	if err != nil {
		slog.Error("Failed to create new group chat", "error", err)
		return 0, err
	}
	return groupID, nil
}

func (ms *MessageService) DeleteGroupChat(ctx context.Context, groupID, userID int) error {
	if err := ms.msgRepo.DeleteGroupChat(ctx, userID, groupID); err != nil {
		slog.Error("Failed to detele group chat", "error", err)
		return err
	}
	return nil
}

func (ms *MessageService) NewGroupMember(ctx context.Context, in *GroupMemberDTO) error {
	slog.Debug("User trying to add other user to group chat",
		"subjectID", in.SubjectID,
		"objectID", in.ObjectID,
		"groupID", in.GroupID,
	)

	messageID, err := ms.msgRepo.NewGroupChatMember(ctx, in.GroupID, in.ObjectID)
	if err != nil {
		slog.Error("Failed to create new group chat member", "error", err)
		return err
	}

	// this is necessary for the client to react to a change in the chat list
	changeListOfGroupsEvent := ChangeListOfGroupsEvent{
		GroupID: in.GroupID,
	}

	changeListOfGroupsEventByte, err := json.Marshal(changeListOfGroupsEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	ms.handleProduce(ctx, in.ObjectID, &ProduceMessage{
		Type: domain.InvitedToGroupChatType,
		Data: changeListOfGroupsEventByte,
	})

	// send event to all users who is in the same chat with object
	newMemberEvent := GroupChangeMemberStatusEvent{
		MessageID: messageID,
		GroupID:   in.GroupID,
		UserID:    in.ObjectID,
	}

	memberIDs, err := ms.msgRepo.GetAllChatMembers(ctx, in.GroupID)
	if err != nil {
		slog.Error("Failed to get all group chat members", "error", err)
		return nil
	}

	newMemberEventByte, err := json.Marshal(newMemberEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	for _, id := range memberIDs {
		if id != in.ObjectID {
			ms.handleProduce(ctx, id, &ProduceMessage{
				Type: domain.NewMemberType,
				Data: newMemberEventByte,
			})
		}
	}
	return nil
}

func (ms *MessageService) DeleteGroupMember(ctx context.Context, in *GroupMemberDTO) error {
	role, err := ms.msgRepo.GetGroupChatMemberRole(ctx, in.SubjectID, in.GroupID)
	if err != nil {
		slog.Error("Failed to get user role in group chat", "error", err)
		return err
	}

	if role != domain.AdminRole {
		slog.Warn("Not admin trying to delete member",
			"subject", in.SubjectID,
			"object", in.ObjectID,
		)
		return domain.ErrForbidden.WithMessage("Change member role can only admin")
	}

	messageID, err := ms.msgRepo.DeleteGroupMember(ctx, in.GroupID, in.ObjectID, *in.Type)
	if err != nil {
		slog.Error("Failed to delete group chat member", "error", err)
		return err
	}

	// if type not is kicked member type it means that client already know about changes
	// and we havent to send event
	if *in.Type == domain.KickedMemberType {
		changeListOfGroupsEvent := ChangeListOfGroupsEvent{
			GroupID: in.GroupID,
		}

		changeListOfGroupsEventByte, err := json.Marshal(changeListOfGroupsEvent)
		if err != nil {
			slog.Error("Failed to marshal data", "error", err)
			return nil
		}

		ms.handleProduce(ctx, in.ObjectID, &ProduceMessage{
			Type: domain.DeletedFromGroupChatType,
			Data: changeListOfGroupsEventByte,
		})
	}

	deleteMemberEvent := GroupChangeMemberStatusEvent{
		MessageID: messageID,
		GroupID:   in.GroupID,
		UserID:    in.ObjectID,
	}

	memberIDs, err := ms.msgRepo.GetAllChatMembers(ctx, in.GroupID)
	if err != nil {
		slog.Error("Failed to get all group chat members", "error", err)
		return err
	}

	kickedMemberEventByte, err := json.Marshal(deleteMemberEvent)
	if err != nil {
		slog.Error("Failed to marshal data", "error", err)
		return nil
	}

	for _, id := range memberIDs {
		if id != in.ObjectID {
			ms.handleProduce(ctx, id, &ProduceMessage{
				Type: *in.Type,
				Data: kickedMemberEventByte,
			})
		}
	}
	return nil
}

func (ms *MessageService) GetAllGroupChatMembers(ctx context.Context, chatID int) ([]int, error) {
	members, err := ms.msgRepo.GetAllChatMembers(ctx, chatID)
	if err != nil {
		slog.Error("Failed to get all group chat members", "error", err)
		return nil, err
	}
	return members, nil
}

func (ms *MessageService) GetUserChats(ctx context.Context, userID int) ([]domain.UserChat, error) {
	chats, err := ms.msgRepo.GetUserChats(ctx, userID)
	if err != nil {
		slog.Error("Failed to get user group chats", "error", err)
		return nil, err
	}
	return chats, nil
}

func (ms *MessageService) ChangeGroupMemberRole(ctx context.Context, in *UpdateGroupMemberRoleDTO) error {
	role, err := ms.msgRepo.GetGroupChatMemberRole(ctx, in.SubjectID, in.ChatID)
	if err != nil {
		slog.Error("Failed to get user role in group chat", "error", err)
		return err
	}

	if role != domain.AdminRole {
		slog.Warn("Not admin trying to change member role",
			"subject", in.SubjectID,
			"object", in.ObjectID,
			"role", string(in.Role),
		)
		return domain.ErrForbidden.WithMessage("Change member role can only admin")
	}

	if err := ms.msgRepo.ChangeGroupChatMemberRole(ctx, in); err != nil {
		slog.Error("Failed to change group member role", "error", err)
		return err
	}
	return nil
}

func (ms *MessageService) PaginateMessages(ctx context.Context, in *PaginateMessagesDTO) ([]domain.Message, *int, bool, error) {
	messages, newCursor, hasMore, err := ms.msgRepo.PaginateMessages(ctx, in.ChatID, in.Cursor)
	if err != nil {
		slog.Error("Failed to paginate chat essages", "error", err)
		return nil, nil, false, err
	}
	return messages, newCursor, hasMore, err
}
