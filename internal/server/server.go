package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/repository"
	"github.com/ReilBleem13/MessangerV2/internal/repository/cache"
	"github.com/ReilBleem13/MessangerV2/internal/repository/database"
	"github.com/ReilBleem13/MessangerV2/internal/service"
)

type Option func(*Server)

func WithMigrateDown(m func() error) Option {
	return func(s *Server) {
		s.migrateDown = m
	}
}

type Server struct {
	router      *http.ServeMux
	migrateDown func() error
}

func NewServer(opts ...Option) *Server {
	s := &Server{
		router: http.NewServeMux(),
	}

	for _, opt := range opts {
		opt(s)
	}

	msgRepository := repository.NewMessageRepo(database.Client(), cache.Client())
	connRepository := repository.NewConnectionRepo(cache.Client())
	msgService := service.NewMessageService(msgRepository, connRepository)

	h := NewHandler(msgService)
	s.setupRoutes(h)

	return s
}

func (s *Server) setupRoutes(h *Handler) {
	s.router.HandleFunc("/ws", h.handleWS)

	s.router.HandleFunc("POST /group", h.handleNewGroup)
	s.router.HandleFunc("DELETE /group/{group_id}", h.handleDeleteGroup)
	s.router.HandleFunc("POST /group/{group_id}/members", h.handleNewGroupMember)
	s.router.HandleFunc("DELETE /group/{group_id}/members", h.handleDeleteGroupMember)
	s.router.HandleFunc("GET /group/{group_id}/members", h.handleGetGroupMembers)
	s.router.HandleFunc("GET /user/{user_id}/groups", h.handleGetUserGroups)
	s.router.HandleFunc("PATCH /users/groups/{group_id}", h.handleUpdateGroupMemberRole)
	s.router.HandleFunc("GET /paginate/private", h.handlePaginatePrivateMessages)
	s.router.HandleFunc("GET /paginate/groups/{group_id}", h.handlePaginateGroupMessages)

	fileServer := http.FileServer(http.Dir("./web"))
	s.router.Handle("/", http.StripPrefix("/", fileServer))
}

func (s *Server) Run(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			return
		}
	}()
	log.Printf("Server is running")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit

	ctx, shutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdown()

	if s.migrateDown != nil {
		if err := s.migrateDown(); err != nil {
			slog.Warn("Failed to migrate down", "error", err)
		}
		slog.Info("Migrations down")
	}

	slog.Info("Server exited")
	return server.Shutdown(ctx)
}
