// Package p2p handles the websocket connection, webrtc connection, and file handling
package p2p

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type P2pManager struct {
	mu          sync.RWMutex
	WC          *websocket.Conn
	PC          *webrtc.PeerConnection
	DC          *webrtc.DataChannel
	ID          string
	DataChan    chan []byte
	MessageChan chan EventMessage
	StatusChan  chan string
	ErrorChan   chan error
	ActivePeer  string
}

func (p *P2pManager) sendStatus(msg string) {
	if p.StatusChan != nil {
		select {
		case p.StatusChan <- msg:
		default:
		}
	}
}

func (p *P2pManager) SendEventMessage(eventType string, msgContent string, target *string, rawData ...json.RawMessage) {
	var targetVal string
	if target != nil {
		targetVal = *target
	}

	event := EventMessage{
		Type:    eventType,
		Message: msgContent,
		Sender:  p.ID,
		Target:  targetVal,
	}
	if len(rawData) > 0 {
		event.Data = rawData[0]
	}

	p.mu.Lock()
	if p.WC == nil {
		p.mu.Unlock()
		return
	}
	err := p.WC.WriteJSON(event)
	p.mu.Unlock()

	if err != nil {
		select {
		case p.ErrorChan <- fmt.Errorf("failed to send event %s: %w", eventType, err):
		default:
			fmt.Printf("[ERROR] Dropped error message to avoid blocking: %v\n", err)
		}
	}
}

func (p *P2pManager) SendPing() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.WC == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	return p.WC.WriteMessage(websocket.PingMessage, nil)
}

func (p *P2pManager) StartListening() {
	defer close(p.MessageChan)

	p.mu.Lock()
	if p.WC != nil {
		p.WC.SetReadDeadline(time.Time{})
	}
	p.mu.Unlock()

	for {
		p.mu.RLock()
		conn := p.WC
		p.mu.RLocker().Unlock()

		if conn == nil {
			return
		}

		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			// Check if it's a genuine unexpected crash vs a standard close window event
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				select {
				case p.ErrorChan <- fmt.Errorf("websocket connection lost unexpectedly: %w", err):
				default:
				}
			}
			return // Break out of the background goroutine cleanly
		}

		var msg EventMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			// Log corrupted data but CONTINUE the loop so the app doesn't freeze
			fmt.Printf("[WARN] Failed to parse signaling payload: %v\n", err)
			continue
		}

		// Ship the parsed event directly to the TUI event listener channel
		select {
		case p.MessageChan <- msg:
		default:
			// Prevent blocking if the TUI update loop is briefly handling UI re-renders
		}
	}
}
