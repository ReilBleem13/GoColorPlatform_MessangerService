package server

import (
	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/ReilBleem13/MessangerV2/internal/service"
)

type NewGroupJSON struct {
	Name string `json:"name"`
}

type NewGroupMember struct {
	UserID int `json:"user_id"`
}

type DeleteGroupMember struct {
	UserID int `json:"user_id"`
}

type UpdateGroupMemberRoleJSON struct {
	Role domain.GroupMemberRole `json:"role"`
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
	Messages  []service.ProduceMessage `json:"messages"`
	NewCursor *int                     `json:"new_cursor,omitempty"`
	HasMore   bool                     `json:"has_more"`
}
