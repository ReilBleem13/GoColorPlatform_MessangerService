package server

import (
	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

type NewGroupJSON struct {
	Name string `json:"name"`
}

type NewGroupMemberJSON struct {
	UserID int `json:"user_id"`
}

// KICKED_MEMBER или LEFT_MEMBER
type DeleteGroupMemberJSON struct {
	Type domain.EventType `json:"type"`
}

type UpdateGroupMemberRoleJSON struct {
	UserID int                    `json:"user_id"`
	Role   domain.GroupMemberRole `json:"role"`
}

type PaginateMessagesJSON struct {
	Cursor *int `json:"cursor,omitempty"`
}

// response
type CreatedGroup struct {
	GroupID int `json:"group_id"`
}

type GroupMembers struct {
	Members []int `json:"members"`
}

type PaginateMessagesResponse struct {
	Messages  []domain.Message `json:"messages"`
	NewCursor *int             `json:"new_cursor,omitempty"`
	HasMore   bool             `json:"has_more"`
}
