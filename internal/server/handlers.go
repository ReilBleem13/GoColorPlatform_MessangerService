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
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		hadleError(w, err)
		return
	}

	client := service.NewClient(userID, conn, service.GetHub())

	h.msgSrv.HandleConn(r.Context(), client)
}

func (h *Handler) handleNewGroup(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in NewGroupJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	groupID, err := h.msgSrv.NewGroup(r.Context(), in.Name, userID)
	if err != nil {
		hadleError(w, err)
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
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.DeleteGroup(r.Context(), groupID, userID); err != nil {
		hadleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handleNewGroupMember(w http.ResponseWriter, r *http.Request) {
	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in NewGroupMember
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.NewGroupMember(r.Context(), groupID, in.UserID); err != nil {
		hadleError(w, err)
		return
	}
	w.WriteHeader(201)
}

func (h *Handler) handleDeleteGroupMember(w http.ResponseWriter, r *http.Request) {
	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in DeleteGroupMember
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	if err := h.msgSrv.DeleteGroupMember(r.Context(), groupID, in.UserID); err != nil {
		hadleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handleGetUserGroups(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	groups, err := h.msgSrv.GetUserGroups(r.Context(), userID)
	if err != nil {
		hadleError(w, err)
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
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	members, err := h.msgSrv.GetAllGroupMembers(r.Context(), groupID)
	if err != nil {
		hadleError(w, err)
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
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in UpdateGroupMemberRoleJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		hadleError(w, err)
		return
	}

	if err := h.msgSrv.ChangeGroupMemberRole(r.Context(), &service.UpdateGroupMemberRoleDTO{
		Role:    in.Role,
		GroupID: groupID,
		UserID:  userID,
	}); err != nil {
		hadleError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) handlePaginatePrivateMessages(w http.ResponseWriter, r *http.Request) {
	userID1Str := r.URL.Query().Get("user_id_1")
	userID1, err := strconv.Atoi(userID1Str)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	userID2Str := r.URL.Query().Get("user_id_2")
	userID2, err := strconv.Atoi(userID2Str)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in PaginateMessagesJSON
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			hadleError(w, err)
			return
		}
	}

	messages, newCursor, hasMore, err := h.msgSrv.PaginatePrivateMessages(r.Context(), &service.PaginatePrivateMessagesDTO{
		User1:  userID1,
		User2:  userID2,
		Cursor: in.Cursor,
	})
	if err != nil {
		hadleError(w, err)
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
	groupIDStr := r.PathValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		hadleError(w, domain.ErrInvalidRequest)
		return
	}

	var in PaginateMessagesJSON
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			hadleError(w, err)
			return
		}
	}

	messages, newCursor, hasMore, err := h.msgSrv.PaginateGroupMessages(r.Context(), &service.PaginateGroupMessagesDTO{
		GroupID: groupID,
		Cursor:  in.Cursor,
	})
	if err != nil {
		hadleError(w, err)
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
