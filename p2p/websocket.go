package p2p

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type EventMessage struct {
	Type    string          `json:"type"`
	Sender  string          `json:"sender"`
	Target  string          `json:"target,omitempty"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ConnectToSignallingServer used to facilitate the WebRTC handshake.
func (p *P2pManager) ConnectToSignallingServer() error {
	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost:8080"
	}
	scheme := "wss"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: host, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	p.WC = conn
	return nil
}

// StartListening continuously reads messages from the WebSocket connection and sends them to the MessageChan. If an error occurs while reading, it sends the error to the ErrorChan and exits.
func (p *P2pManager) StartListening() {
	defer close(p.MessageChan)

	for {
		_, rawMsg, err := p.WC.ReadMessage()
		if err != nil {
			p.ErrorChan <- fmt.Errorf("connection closed or read error: %w", err)
			return
		}

		var msg EventMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			p.ErrorChan <- fmt.Errorf("failed to unmarshal message: %w", err)
			continue
		}
		p.MessageChan <- msg
	}
}

// SendEventMessage sends an event message with the specified type, content, target, and optional raw data over the WebSocket connection. If an error occurs while sending, it sends the error to the ErrorChan.
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
	err := p.WC.WriteJSON(event)
	p.mu.Unlock()

	if err != nil {
		select {
		case p.ErrorChan <- err:
		default:
			fmt.Printf("[ERROR] Failed to send event %s: %v\n", eventType, err)
		}
	}
}
