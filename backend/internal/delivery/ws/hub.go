package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"catbrasil/backend/pkg/jwt"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"bearer"},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Clientes não-browser ainda precisam de JWT válido.
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return false
		}
		if configured := os.Getenv("CORS_ALLOWED_ORIGINS"); configured != "" {
			for _, allowed := range strings.Split(configured, ",") {
				if strings.TrimSpace(allowed) == origin {
					return true
				}
			}
			return false
		}
		return parsed.Host == r.Host
	},
}

// WebSocketMessage representa a estrutura de um evento transmitido via WebSocket
type WebSocketMessage struct {
	Event   string      `json:"event"`
	Payload interface{} `json:"payload"`
	UserID  string      `json:"userId,omitempty"`
}

// Client é a conexão individual de um operador conectado
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	userID    string
	expiresAt time.Time
}

// Hub gerencia o conjunto de conexões ativas e roteia eventos em tempo real
type Hub struct {
	clients     map[*Client]bool
	userClients map[string]map[*Client]bool
	broadcast   chan WebSocketMessage
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
}

// NewHub inicializa uma nova instância do Hub de WebSockets
func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		userClients: make(map[string]map[*Client]bool),
		broadcast:   make(chan WebSocketMessage, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
	}
}

// Run inicia o loop de eventos assíncrono do Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.userID != "" {
				if h.userClients[client.userID] == nil {
					h.userClients[client.userID] = make(map[*Client]bool)
				}
				h.userClients[client.userID][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if client.userID != "" && h.userClients[client.userID] != nil {
					delete(h.userClients[client.userID], client)
					if len(h.userClients[client.userID]) == 0 {
						delete(h.userClients, client.userID)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			data, err := json.Marshal(message)
			if err != nil {
				continue
			}

			h.mu.Lock()
			if message.UserID != "" {
				if clients, ok := h.userClients[message.UserID]; ok {
					for client := range clients {
						select {
						case client.send <- data:
						default:
							close(client.send)
							delete(h.clients, client)
							delete(clients, client)
						}
					}
				}
			} else {
				for client := range h.clients {
					select {
					case client.send <- data:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// NotifyUser envia um evento em tempo real exclusivamente para um operador conectado
func (h *Hub) NotifyUser(userID string, event string, payload interface{}) {
	if userID == "" {
		return
	}
	h.broadcast <- WebSocketMessage{
		Event:   event,
		Payload: payload,
		UserID:  userID,
	}
}

// Broadcast envia um comunicado global para todos os clientes conectados
func (h *Hub) Broadcast(event string, payload interface{}) {
	h.broadcast <- WebSocketMessage{
		Event:   event,
		Payload: payload,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Erro na leitura do WebSocket: %v", err)
			}
			break
		}

		// Mensagens do cliente não alteram a identidade definida pelo JWT.
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	expiry := time.NewTimer(time.Until(c.expiresAt))
	defer func() {
		ticker.Stop()
		expiry.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-expiry.C:
			return
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWS retorna o HTTP HandlerFunc para upgrade da conexão para WebSocket
func (h *Hub) HandleWS(tokenManager *jwt.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string
		protocols := websocket.Subprotocols(r)
		if len(protocols) == 2 && protocols[0] == "bearer" {
			tokenStr = protocols[1]
		}
		if tokenStr == "" || tokenManager == nil {
			http.Error(w, "Token de autenticação obrigatório", http.StatusUnauthorized)
			return
		}
		claims, err := tokenManager.ValidateToken(tokenStr)
		if err != nil || claims == nil || claims.UserID == "" || claims.ExpiresAt == nil {
			http.Error(w, "Token de autenticação inválido", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Falha ao efetuar upgrade WebSocket: %v", err)
			return
		}

		client := &Client{
			hub:       h,
			conn:      conn,
			send:      make(chan []byte, 256),
			userID:    claims.UserID,
			expiresAt: claims.ExpiresAt.Time,
		}

		client.hub.register <- client

		go client.writePump()
		go client.readPump()
	}
}
