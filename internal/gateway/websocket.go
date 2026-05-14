package gateway

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sui-nexus/gateway/internal/model"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan *model.Task
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan *model.Task, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()
		case task := <-h.broadcast:
			h.mu.RLock()
			var dead []*websocket.Conn
			for conn := range h.clients {
				if err := conn.WriteJSON(task); err != nil {
					log.Printf("WebSocket write error: %v", err)
					conn.Close()
					dead = append(dead, conn)
				}
			}
			h.mu.RUnlock()
			if len(dead) > 0 {
				h.mu.Lock()
				for _, conn := range dead {
					delete(h.clients, conn)
				}
				h.mu.Unlock()
			}
		}
	}
}

func (h *Hub) BroadcastTask(task *model.Task) {
	select {
	case h.broadcast <- task:
	default:
	}
}

func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	h.hub.register <- conn
	defer func() {
		h.hub.unregister <- conn
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
