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
			// Clean up partially downloaded files if quitting early
			if m.fileWriter != nil {
				m.fileWriter.Close()
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Reset):
			return m.InitialModel(m.p2p), textinput.Blink

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
		if msg.isInitiator {
			cmds = append(cmds, cmdSendOffer(m.p2p, m.totpInput.Value()))
		}
		// Both sides should start listening for incoming DataChannel messages
		cmds = append(cmds, listenForData(m.p2p.DataChan))
		// (Assume you also have a listener for signaling events here)

	case offerSentMsg:
		m.logs = append(m.logs, "[Sender] Offer sent via signaling. Waiting for Answer...")
		// Now we wait for signalingEventMsg containing the answer

	case answerHandledMsg:
		m.logs = append(m.logs, "[Sender] Answer received. P2P Tunnel open!")
		m.logs = append(m.logs, "[Sender] Chunking file and sending manifest...")
		cmds = append(cmds, cmdChunkAndSendManifest(m.p2p, m.pathInput.Value()))

	case manifestSentMsg:
		m.logs = append(m.logs, "[Sender] Manifest sent. Sending chunks...")
		m.progressChan = make(chan float64) // Add this to your Model
		cmds = append(
			cmds,
			cmdSendChunks(m.p2p, m.pathInput.Value(), m.progressChan),
			listenForProgress(m.progressChan),
		)

	case chunkProgressMsg:
		m.logs = append(m.logs, fmt.Sprintf("[Transfer] Sending... %.1f%%", msg.percent))
		// Keep listening for the next progress update
		cmds = append(cmds, listenForProgress(m.progressChan))

	// ---------------------------------------------------------
	// Receiver Logic: Parsing DataChannel Messages
	// ---------------------------------------------------------
	case dataReceivedMsg:
		// 1. Check if we are expecting a manifest (first message received)
		if !m.manifestReceived {
			var incomingManifest Manifest
			err := json.Unmarshal(msg.data, &incomingManifest)
			if err == nil && incomingManifest.Chunks > 0 {
				m.manifestReceived = true
				m.manifest = incomingManifest
				m.logs = append(m.logs, fmt.Sprintf("[Receiver] Manifest received: %s (%.2f MB)", m.manifest.Filename, float64(m.manifest.Size)/1024/1024))

				// Prepare the file for writing
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
			// 2. We already have the manifest, so this must be a raw file chunk
			if m.fileWriter != nil {
				_, err := m.fileWriter.Write(msg.data)
				if err != nil {
					m.logs = append(m.logs, "[ERROR] Failed to write chunk to disk: "+err.Error())
				}

				m.receivedChunks++
				progress := (float64(m.receivedChunks) / float64(m.manifest.Chunks)) * 100

				// Optional: only log every 10% to prevent TUI log spam
				if m.receivedChunks%50 == 0 || m.receivedChunks == m.manifest.Chunks {
					m.logs = append(m.logs, fmt.Sprintf("[Receiver] Receiving... %.1f%%", progress))
				}

				if m.receivedChunks == m.manifest.Chunks {
					m.fileWriter.Close()
					m.fileWriter = nil
					cmds = append(cmds, func() tea.Msg { return transferCompleteMsg{} })
				}
			}
		}
		// Continue listening for the next message on the DataChannel
		if m.receivedChunks < m.manifest.Chunks {
			cmds = append(cmds, listenForData(m.p2p.DataChan))
		}

	case transferCompleteMsg:
		m.logs = append(m.logs, "[Success] File transfer complete! Closing connection...")
		cmds = append(cmds, cmdCloseConnection(m.p2p))

	case connectionClosedMsg:
		m.logs = append(m.logs, "[System] Connection closed cleanly. Press 'n' to reset or 'q' to quit.")
	}

	// Update text inputs based on focus
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
