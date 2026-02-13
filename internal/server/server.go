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

	"github.com/ReilBleem13/MessangerV2/internal/config"
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
	ctx         context.Context
	router      *http.ServeMux
	cfg         *config.Config
	migrateDown func() error
}

func NewServer(ctx context.Context, opts ...Option) *Server {
	s := &Server{
		router: http.NewServeMux(),
	}

	for _, opt := range opts {
		opt(s)
	}

	msgRepository := repository.NewMessageRepo(database.Client(), cache.Client())
	connRepository := repository.NewConnectionRepo(cache.Client())

	heartbeatService := service.NewHeartbeatService(ctx, connRepository)
	msgService := service.NewMessageService(heartbeatService, msgRepository, connRepository)

	h := NewHandler(msgService)
	s.setupRoutes(h)

	return s
}

func (s *Server) setupRoutes(h *Handler) {
	s.router.HandleFunc("/ws", h.handleWS)

	authMiddleware := AuthMiddleware(s.cfg.JWT.Secret)

	s.router.Handle("POST /chats", authMiddleware(http.HandlerFunc(h.handleNewGroup)))
	s.router.Handle("DELETE /chats/{chat_id}", authMiddleware(http.HandlerFunc(h.handleDeleteGroup)))
	s.router.Handle("POST /chats/{chat_id}/members", authMiddleware(http.HandlerFunc(h.handleNewGroupMember)))
	s.router.Handle("DELETE /chats/{chat_id}/members/{user_id}", authMiddleware(http.HandlerFunc(h.handleDeleteGroupMember)))
	s.router.Handle("PATCH /chats/{chat_id}/members/{user_id}", authMiddleware(http.HandlerFunc(h.handleUpdateGroupMemberRole)))
	s.router.Handle("GET /chats/{chat_id}/members", authMiddleware(http.HandlerFunc(h.handleGetGroupMembers)))

	s.router.Handle("GET /users/chats", authMiddleware(http.HandlerFunc(h.handleGetUserGroups)))
	s.router.Handle("GET /users/chat/{chat_id}", authMiddleware(http.HandlerFunc(h.handlePaginateMessages)))

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

	ctx, shutdown := context.WithTimeout(s.ctx, 10*time.Second)
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
