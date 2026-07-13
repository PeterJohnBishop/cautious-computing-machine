package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			if m.fileWriter != nil {
				m.fileWriter.Close()
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Reset):
			// FIX 1: Removed the "m." prefix
			return InitialModel(m.p2p), textinput.Blink

		case key.Matches(msg, keys.Left):
			if m.focusIndex == FocusToggle {
				m.role = RoleSender
				m.updatePlaceholders()
			}
		case key.Matches(msg, keys.Right):
			if m.focusIndex == FocusToggle {
				m.role = RoleReceiver
				m.updatePlaceholders()
			}

		case key.Matches(msg, keys.Tab, keys.Enter):
			if m.focusIndex < FocusConnected {
				if m.role == RoleSender && m.focusIndex == FocusPath {
					m.focusIndex = FocusConnected
					cmds = append(cmds, m.startConnectionSequence())
				} else {
					m.focusIndex++
					if m.focusIndex == FocusConnected {
						cmds = append(cmds, m.startConnectionSequence())
					}
				}
				m.updateFocusStates()
			}

		case key.Matches(msg, keys.ShiftTab):
			if m.focusIndex > FocusToggle {
				if m.role == RoleSender && m.focusIndex == FocusConnected {
					m.focusIndex = FocusPath
				} else {
					m.focusIndex--
				}
				m.updateFocusStates()
			}
		}

	case logMsg:
		m.logs = append(m.logs, string(msg))
		cmds = append(cmds, listenForStatus(m.p2p.StatusChan))

	case errMsg:
		m.logs = append(m.logs, "[ERROR] "+msg.err.Error())
		cmds = append(cmds, listenForErrors(m.p2p.ErrorChan))

	case wsConnectedMsg:
		m.logs = append(m.logs, "[System] WebSocket connected. Starting WebRTC...")
		isInitiator := m.role == RoleSender
		cmds = append(cmds, cmdStartWebRTC(m.p2p, isInitiator))

	case webrtcStartedMsg:
		cmds = append(
			cmds,
			listenForData(m.p2p.DataChan),
			listenForSignaling(m.p2p.MessageChan),
		)

		if msg.isInitiator {
			m.logs = append(m.logs, "[Sender] Waiting for receiver to connect...")
		} else {
			m.logs = append(m.logs, "[Receiver] Knocking on sender's door...")
			target := m.p2p.ActivePeer
			m.p2p.SendEventMessage("peer_joined", "Hello, I am ready to receive!", &target)
		}

	case signalingEventMsg:
		switch msg.eventType {
		case "peer_joined":
			if m.role == RoleSender {
				m.logs = append(m.logs, fmt.Sprintf("[Sender] Receiver %s joined! Sending Offer...", msg.sender))
				// Lock onto the receiver's unique ID
				m.p2p.ActivePeer = msg.sender
				cmds = append(cmds, cmdSendOffer(m.p2p, msg.sender))
			}

		case "offer":
			m.logs = append(m.logs, "[Receiver] Offer received. Processing...")
			m.p2p.ActivePeer = msg.sender
			cmds = append(cmds, cmdHandleOffer(m.p2p, msg.sender, string(msg.payload)))

		case "answer":
			m.logs = append(m.logs, "[Sender] Answer received via signaling...")
			cmds = append(cmds, cmdHandleAnswer(m.p2p, string(msg.payload)))

		case "candidate":
			cmds = append(cmds, cmdHandleICECandidate(m.p2p, msg.payload))
		}

		// Loop the listener to catch the next incoming signaling event
		cmds = append(cmds, listenForSignaling(m.p2p.MessageChan))

	case offerSentMsg:
		m.logs = append(m.logs, "[Sender] Offer sent via signaling. Waiting for Answer...")

	case offerHandledMsg:
		m.logs = append(m.logs, "[Receiver] Offer accepted. Answer sent.")
		m.logs = append(m.logs, "[Receiver] Waiting for P2P tunnel to open and manifest to arrive...")

	case answerHandledMsg:
		m.logs = append(m.logs, "[Sender] Handshake complete. P2P Tunnel open!")
		m.logs = append(m.logs, "[Sender] Chunking file and sending manifest...")
		cmds = append(cmds, cmdChunkAndSendManifest(m.p2p, m.pathInput.Value()))

	case manifestSentMsg:
		m.logs = append(m.logs, "[Sender] Manifest sent. Sending chunks...")
		m.progressChan = make(chan float64)
		cmds = append(
			cmds,
			cmdSendChunks(m.p2p, m.pathInput.Value(), m.progressChan),
			listenForProgress(m.progressChan),
		)

	case chunkProgressMsg:
		m.logs = append(m.logs, fmt.Sprintf("[Transfer] Sending... %.1f%%", msg.percent))
		cmds = append(cmds, listenForProgress(m.progressChan))

	case dataReceivedMsg:
		if !m.manifestReceived {
			var incomingManifest Manifest
			err := json.Unmarshal(msg.data, &incomingManifest)
			if err == nil && incomingManifest.Chunks > 0 {
				m.manifestReceived = true
				m.manifest = incomingManifest
				m.logs = append(m.logs, fmt.Sprintf("[Receiver] Manifest received: %s (%.2f MB)", m.manifest.Filename, float64(m.manifest.Size)/1024/1024))

				savePath := filepath.Join(m.pathInput.Value(), m.manifest.Filename)
				file, err := os.Create(savePath)
				if err != nil {
					m.logs = append(m.logs, "[ERROR] Could not create file: "+err.Error())
				} else {
					m.fileWriter = file
				}
			} else {
				m.logs = append(m.logs, "[ERROR] Failed to parse file manifest.")
			}
		} else {
			if m.fileWriter != nil {
				_, err := m.fileWriter.Write(msg.data)
				if err != nil {
					m.logs = append(m.logs, "[ERROR] Failed to write chunk to disk: "+err.Error())
				}

				m.receivedChunks++
				progress := (float64(m.receivedChunks) / float64(m.manifest.Chunks)) * 100

				if m.receivedChunks%(m.manifest.Chunks/10+1) == 0 || m.receivedChunks == m.manifest.Chunks {
					m.logs = append(m.logs, fmt.Sprintf("[Receiver] Receiving... %.1f%%", progress))
				}

				if m.receivedChunks == m.manifest.Chunks {
					m.fileWriter.Close()
					m.fileWriter = nil
					cmds = append(cmds, func() tea.Msg { return transferCompleteMsg{} })
				}
			}
		}

		if m.receivedChunks < m.manifest.Chunks {
			cmds = append(cmds, listenForData(m.p2p.DataChan))
		}

	case transferCompleteMsg:
		m.logs = append(m.logs, "[Success] File transfer complete! Closing connection...")
		cmds = append(cmds, cmdCloseConnection(m.p2p))

	case connectionClosedMsg:
		m.logs = append(m.logs, "[System] Connection closed cleanly. Press 'n' to reset or 'q' to quit.")
	}

	switch m.focusIndex {
	case FocusPath:
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		cmds = append(cmds, cmd)
	case FocusTOTP:
		var cmd tea.Cmd
		m.totpInput, cmd = m.totpInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
