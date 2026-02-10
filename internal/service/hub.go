package service

import "log/slog"

var hub = NewHub()

type Hub struct {
	clients map[int]*Client

	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func GetHub() *Hub {
	return hub
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client.id] = client
			slog.Info("User Connected", "user_id", client.id)

		case client := <-h.unregister:
			delete(h.clients, client.id)
			slog.Info("User disconnected", "user_id", client.id)
		}
	}
}
