// Package tui handles the user interface
package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/peterjohnbishop/cautious-computing-machine/p2p"
)

var (
	activeColor   = lipgloss.Color("212")
	inactiveColor = lipgloss.Color("241")
	logStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).MarginLeft(2)
	titleStyle    = lipgloss.NewStyle().Foreground(activeColor).Bold(true).MarginBottom(1)
	activeStyle   = lipgloss.NewStyle().Foreground(activeColor).Bold(true)
	inactiveStyle = lipgloss.NewStyle().Foreground(inactiveColor)
)

type keyMap struct {
	Left, Right, Enter, Tab, ShiftTab, Quit, Reset key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return []key.Binding{k.Tab, k.ShiftTab, k.Reset, k.Quit} }
func (k keyMap) FullHelp() [][]key.Binding { return nil }

var keys = keyMap{
	Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Reset:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "reset")),
}

const (
	RoleSender = iota
	RoleReceiver
)

const (
	FocusToggle = iota
	FocusPath
	FocusTOTP
	FocusConnected
)

type (
	logMsg string
	errMsg struct{ err error }
)

type (
	wsConnectedMsg      struct{}
	webrtcStartedMsg    struct{ isInitiator bool }
	offerSentMsg        struct{}
	offerHandledMsg     struct{}
	answerHandledMsg    struct{}
	manifestSentMsg     struct{}
	chunkSentMsg        struct{ progress string }
	transferCompleteMsg struct{}
	connectionClosedMsg struct{}
)

func listenForStatus(sub chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-sub
		if !ok {
			return nil // channel closed
		}
		return logMsg(msg)
	}
}

func listenForErrors(sub chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-sub
		if !ok {
			return nil
		}
		return errMsg{err: err}
	}
}

func (m *Model) updatePlaceholders() {
	if m.role == RoleSender {
		m.pathInput.Placeholder = "file path"
	} else {
		m.pathInput.Placeholder = "save file directory"
	}
}

func (m *Model) updateFocusStates() {
	m.pathInput.Blur()
	m.totpInput.Blur()

	switch m.focusIndex {
	case FocusPath:
		m.pathInput.Focus()
	case FocusTOTP:
		m.totpInput.Focus()
	}
}

func (m *Model) startConnectionSequence() tea.Cmd {
	targetID := m.totpInput.Value()
	if m.role == RoleSender {
		targetID = m.totpSecret
	}

	return tea.Batch(
		func() tea.Msg {
			return logMsg(fmt.Sprintf("[System] Connecting to signaling server with ID: %s", targetID))
		},
		cmdConnectWS(targetID),
	)
}

func cmdConnectWS(id string) tea.Cmd {
	return func() tea.Msg {
		// TODO: p.ConnectWS(id)
		time.Sleep(800 * time.Millisecond) // Placeholder delay
		return wsConnectedMsg{}
	}
}

func cmdStartWebRTC(p *p2p.P2pManager, isInitiator bool) tea.Cmd {
	return func() tea.Msg {
		if err := p.StartWebRTC(isInitiator); err != nil {
			return errMsg{err: fmt.Errorf("failed to start WebRTC: %w", err)}
		}
		return webrtcStartedMsg{isInitiator: isInitiator}
	}
}

func cmdSendOffer(p *p2p.P2pManager, targetID string) tea.Cmd {
	return func() tea.Msg {
		if err := p.SendOffer(targetID); err != nil {
			return errMsg{err: fmt.Errorf("failed to send offer: %w", err)}
		}
		return offerSentMsg{}
	}
}

func cmdHandleOffer() tea.Cmd {
	return func() tea.Msg {
		// TODO: p.HandleOffer()
		time.Sleep(1 * time.Second)
		return offerHandledMsg{}
	}
}

func cmdHandleAnswer() tea.Cmd {
	return func() tea.Msg {
		// TODO: p.HandleAnswer()
		time.Sleep(1 * time.Second)
		return answerHandledMsg{}
	}
}

func cmdChunkAndSendManifest(filepath string) tea.Cmd {
	return func() tea.Msg {
		// TODO: File stat generation, chunking logic, p.SafeWriteBytesToDC()
		time.Sleep(600 * time.Millisecond)
		return manifestSentMsg{}
	}
}

func cmdSendChunks() tea.Cmd {
	return func() tea.Msg {
		// TODO: Loop through file chunks and stream via WebRTC
		time.Sleep(400 * time.Millisecond)
		return chunkSentMsg{progress: "100%"} // replace with actual progressive progress updates
	}
}

func cmdCloseConnection(p *p2p.P2pManager) tea.Cmd {
	return func() tea.Msg {
		p.DisconnectWebRTC()
		return connectionClosedMsg{}
	}
}
