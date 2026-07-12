// Package p2p handles the websocket connection, webrtc connection, and file handling
package p2p

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type p2pManager struct {
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

func (p *p2pManager) sendStatus(msg string) {
	if p.StatusChan != nil {
		select {
		case p.StatusChan <- msg:
		default:
		}
	}
}
