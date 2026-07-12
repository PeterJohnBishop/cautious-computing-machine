package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg: // V2 change: KeyMsg is now KeyPressMsg
		switch {
		case key.Matches(msg, keys.Quit):
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
	case offerSentMsg:
		m.logs = append(m.logs, "[Sender] Offer sent via signaling. Waiting for Answer...")
		cmds = append(cmds, cmdHandleAnswer())

	case offerHandledMsg:
		m.logs = append(m.logs, "[Receiver] Offer accepted. Answer sent. P2P Tunnel open!")
		m.logs = append(m.logs, "[Receiver] Waiting for file manifest...")

	case answerHandledMsg:
		m.logs = append(m.logs, "[Sender] Answer received. P2P Tunnel open!")
		m.logs = append(m.logs, "[Sender] Chunking file and sending manifest...")
		cmds = append(cmds, cmdChunkAndSendManifest(m.pathInput.Value()))

	case manifestSentMsg:
		m.logs = append(m.logs, "[Sender] Manifest sent. Sending chunks...")
		cmds = append(cmds, cmdSendChunks())

	case chunkSentMsg:
		m.logs = append(m.logs, fmt.Sprintf("[Transfer] Sending... %s", msg.progress))

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
