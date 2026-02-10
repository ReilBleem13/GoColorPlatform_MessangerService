package service

import (
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	id       int
	conn     *websocket.Conn
	outboard <-chan *redis.Message
	hub      *Hub
}

func NewClient(id int, conn *websocket.Conn, hub *Hub) *Client {
	client := &Client{
		id:   id,
		conn: conn,
		hub:  hub,
	}

	hub.register <- client
	return client
}
