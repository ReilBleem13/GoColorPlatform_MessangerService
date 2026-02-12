package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/ReilBleem13/MessangerV2/internal/service"
	"github.com/gorilla/websocket"
)

type Handler struct {
	msgSrv   service.MessageServiceIn
	upgrader *websocket.Upgrader
}

func NewHandler(msgSrv service.MessageServiceIn) *Handler {
	return &Handler{
		msgSrv: msgSrv,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferPool: &sync.Pool{},
		},
	}
}

func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	client := service.NewClient(userID, conn, service.GetHub())
	h.msgSrv.HandleConn(r.Context(), client)
}

func (h *Handler) handleNewGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	var in NewGroupJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	groupID, err := h.msgSrv.NewGroup(r.Context(), in.Name, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	resp := &CreatedGroup{
		GroupID: groupID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.DeleteGroup(r.Context(), groupID, userID); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handleNewGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	var in NewGroupMemberJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.NewGroupMember(r.Context(), &service.GroupMemberDTO{
		GroupID:   groupID,
		SubjectID: userID,
		ObjectID:  in.UserID,
	}); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(201)
}

func (h *Handler) handleDeleteGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	userIDToDeleteStr := r.PathValue("user_id")
	userIDToDelete, err := strconv.Atoi(userIDToDeleteStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	var in DeleteGroupMemberJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.DeleteGroupMember(r.Context(), &service.GroupMemberDTO{
		GroupID:   groupID,
		SubjectID: userID,
		ObjectID:  userIDToDelete,
		Type:      &in.Type,
	}); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handleGetUserGroups(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groups, err := h.msgSrv.GetUserGroups(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(groups)
}

func (h *Handler) handleGetGroupMembers(w http.ResponseWriter, r *http.Request) {
	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	members, err := h.msgSrv.GetAllGroupMembers(r.Context(), groupID)
	if err != nil {
		handleError(w, err)
		return
	}

	resp := &GroupMembers{
		Members: members,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleUpdateGroupMemberRole(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	var in UpdateGroupMemberRoleJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		handleError(w, err)
		return
	}

	if err := h.msgSrv.ChangeGroupMemberRole(r.Context(), &service.UpdateGroupMemberRoleDTO{
		Role:    in.Role,
		GroupID: groupID,
		UserID:  userID,
	}); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handlePaginatePrivateMessages(w http.ResponseWriter, r *http.Request) {
	chatIDStr := r.PathValue("chat_id")
	chatID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	var in PaginateMessagesJSON
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			handleError(w, err)
			return
		}
	}

	messages, newCursor, hasMore, err := h.msgSrv.PaginatePrivateMessages(r.Context(), &service.PaginatePrivateMessagesDTO{
		User1:  userID1,
		User2:  userID2,
		Cursor: in.Cursor,
	})
	if err != nil {
		handleError(w, err)
		return
	}

	resp := &PaginateMessagesResponse{
		Messages:  messages,
		NewCursor: newCursor,
		HasMore:   hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handlePaginateGroupMessages(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		handleError(w, domain.ErrInternalServerError)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		handleError(w, domain.ErrInvalidRequest)
		return
	}

	var in PaginateMessagesJSON
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			handleError(w, domain.ErrInvalidRequest)
			return
		}
	}

	messages, newCursor, hasMore, err := h.msgSrv.PaginateGroupMessages(r.Context(), &service.PaginateGroupMessagesDTO{
		UserID:  userID,
		GroupID: groupID,
		Cursor:  in.Cursor,
	})
	if err != nil {
		handleError(w, err)
		return
	}

	resp := &PaginateMessagesResponse{
		Messages:  messages,
		NewCursor: newCursor,
		HasMore:   hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(resp)
}
