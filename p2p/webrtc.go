package p2p

import (
	"encoding/json"
	"fmt"

	"github.com/pion/webrtc/v4"
)

func (p *P2pManager) StartWebRTC(isSender bool) error {
	p.mu.RLock()
	wc := p.WC
	p.mu.RUnlock()

	if wc == nil {
		return fmt.Errorf("connection manager must have an initialized websocket")
	}

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	p.mu.Lock()
	p.PC = pc
	p.mu.Unlock()

	if isSender {
		dc, err := pc.CreateDataChannel("dataTransfer", nil)
		if err != nil {
			return fmt.Errorf("failed to create data channel: %w", err)
		}

		p.mu.Lock()
		p.DC = dc
		p.mu.Unlock()

		dc.OnOpen(func() {
			p.sendStatus("Local Data channel is open. Sending...")
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if p.DataChan != nil {
				p.DataChan <- msg.Data
			}
		})
	} else {
		pc.OnDataChannel(func(d *webrtc.DataChannel) {
			p.mu.Lock()
			p.DC = d
			p.mu.Unlock()

			d.OnOpen(func() {
				p.sendStatus("Remote Data channel opened. Ready to receive...")
			})

			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				if p.DataChan != nil {
					// Push incoming data to the TUI receiver loop
					p.DataChan <- msg.Data
				}
			})
		})
	}

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateJSON := candidate.ToJSON()
		candidateBytes, err := json.Marshal(candidateJSON)
		if err != nil {
			p.sendStatus(fmt.Sprintf("Failed to marshal ICE candidate: %v", err))
			return
		}

		p.mu.RLock()
		target := p.ActivePeer
		p.mu.RUnlock()

		p.SendEventMessage("candidate", "ICE Candidate", &target, candidateBytes)
	})

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		p.sendStatus(fmt.Sprintf("ICE Connection State has changed: %s", connectionState.String()))

		if connectionState == webrtc.ICEConnectionStateConnected {
			p.sendStatus("Peers connected!")
		}
	})

	p.sendStatus("WebRTC is ready to connect. Searching for ICE candidates...")
	return nil
}

func (p *P2pManager) HandleICECandidate(candidateBytes []byte) error {
	p.mu.RLock()
	pc := p.PC
	p.mu.RUnlock()

	if pc == nil {
		return fmt.Errorf("peer connection must be initialized before adding candidates")
	}

	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateBytes, &candidate); err != nil {
		return fmt.Errorf("failed to unmarshal remote ICE candidate: %w", err)
	}

	if err := pc.AddICECandidate(candidate); err != nil {
		return fmt.Errorf("failed to add remote ICE candidate: %w", err)
	}

	p.sendStatus("Remote ICE candidate applied successfully.")
	return nil
}

func (p *P2pManager) SendOffer(target string) error {
	p.mu.RLock()
	pc := p.PC
	p.mu.RUnlock()

	if pc == nil {
		return fmt.Errorf("peer connection is nil. call StartWebRTC first")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create an offer: %w", err)
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	offerBytes, err := json.Marshal(offer)
	if err != nil {
		return fmt.Errorf("failed to marshal offer: %w", err)
	}

	p.SendEventMessage("offer", "WebRTC", &target, offerBytes)
	p.sendStatus("outbound offer generated and sent to signaling server.")
	return nil
}

func (p *P2pManager) HandleOffer(sender string, offerBytes []byte) error {
	p.mu.RLock()
	pc := p.PC
	p.mu.RUnlock()

	if pc == nil {
		return fmt.Errorf("peer connection must be initialized")
	}

	var offer webrtc.SessionDescription
	if err := json.Unmarshal(offerBytes, &offer); err != nil {
		return fmt.Errorf("failed to unmarshal remote offer: %w", err)
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		return fmt.Errorf("failed to set the session description: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("failed to create an answer: %w", err)
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("failed to set the local description: %w", err)
	}

	answerBytes, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("failed to marshal answer: %w", err)
	}

	p.SendEventMessage("answer", "WebRTC Answer", &sender, answerBytes)
	p.sendStatus("offer accepted. outbound answer sent.")
	return nil
}

func (p *P2pManager) HandleAnswer(answerBytes []byte) error {
	p.mu.RLock()
	pc := p.PC
	p.mu.RUnlock()

	if pc == nil {
		return fmt.Errorf("peer connection must be initialized")
	}

	var answer webrtc.SessionDescription
	if err := json.Unmarshal(answerBytes, &answer); err != nil {
		return fmt.Errorf("failed to unmarshal remote answer: %w", err)
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("failed to apply remote answer: %w", err)
	}

	p.sendStatus("handshake complete for P2P tunnel")
	return nil
}

func (p *P2pManager) SafeWriteBytesToDC(data []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.DC == nil {
		return fmt.Errorf("data channel is not initialized")
	}

	return p.DC.Send(data)
}

func (p *P2pManager) DisconnectWebRTC() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.DC != nil {
		p.DC.Close()
	}
	if p.PC != nil {
		p.PC.Close()
	}
	p.DC = nil
	p.PC = nil
	p.sendStatus("WebRTC connection closed")
}
