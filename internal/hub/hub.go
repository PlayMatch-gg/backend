package hub

import (
	"encoding/json"
	"sync"
)

// Event represents a real-time event to be sent to clients.
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client represents a single client connection (a user in a lobby).
// It's essentially a channel that the SSE handler will listen to.
type Client chan []byte

// Hub manages all active lobbies and their clients.
type Hub struct {
	lobbies map[uint]map[Client]bool
	mu      sync.RWMutex
}

// GlobalHub is the singleton instance of our Hub.
var GlobalHub = NewHub()

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		lobbies: make(map[uint]map[Client]bool),
	}
}

// Subscribe adds a new client to a specific lobby.
func (h *Hub) Subscribe(lobbyID uint, client Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.lobbies[lobbyID]; !ok {
		h.lobbies[lobbyID] = make(map[Client]bool)
	}
	h.lobbies[lobbyID][client] = true
}

// Unsubscribe removes a client from a lobby.
func (h *Hub) Unsubscribe(lobbyID uint, client Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.lobbies[lobbyID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client) // Close the channel to signal the SSE handler to stop.
			if len(clients) == 0 {
				delete(h.lobbies, lobbyID)
			}
		}
	}
}

// Broadcast sends an event to all clients in a specific lobby.
func (h *Hub) Broadcast(lobbyID uint, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.lobbies[lobbyID]; ok {
		messageBytes, err := json.Marshal(event)
		if err != nil {
			// Handle error appropriately, maybe log it
			return
		}
		
		for client := range clients {
			// Use a non-blocking send to prevent a slow client from blocking the hub.
			select {
			case client <- messageBytes:
			default:
				// Client channel is full, maybe they are disconnected or slow.
				// The unsubscribe logic will handle cleaning this up eventually.
			}
		}
	}
}
