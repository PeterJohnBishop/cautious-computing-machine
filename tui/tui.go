// Package tui handles the user interface
package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
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
	signalingEventMsg   struct {
		eventType string // "offer", "answer", etc.
		payload   []byte
	}
	dataReceivedMsg  struct{ data []byte }
	chunkProgressMsg struct{ percent float64 }
	writeErrorMsg    struct{ err error }
)

type Manifest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Chunks   int    `json:"chunks"`
}

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

// listenForSignaling waits for WebRTC handshakes (Offers/Answers) from your WebSocket.
func listenForSignaling(signalChan chan []byte) tea.Cmd {
	return func() tea.Msg {
		data, ok := <-signalChan
		if !ok {
			return nil
		}
		// Assuming payload dictates if it's an offer or answer
		return signalingEventMsg{payload: data}
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
	m.p2p.ID = m.totpInput.Value()
	if m.role == RoleSender {
		m.p2p.ID = m.totpSecret
	}

	return tea.Batch(
		func() tea.Msg {
			return logMsg(fmt.Sprintf("[System] Connecting to signaling server with ID: %s", m.p2p.ID))
		},
		m.cmdConnectWS(),
	)
}

func (m *Model) cmdConnectWS() tea.Cmd {
	return func() tea.Msg {
		err := m.p2p.ConnectToSignallingServer()
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to connect to the signaling server: %w", err)}
		}
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

func cmdCloseConnection(p *p2p.P2pManager) tea.Cmd {
	return func() tea.Msg {
		p.DisconnectWebRTC()
		return connectionClosedMsg{}
	}
}

func listenForData(dataChan chan []byte) tea.Cmd {
	return func() tea.Msg {
		data, ok := <-dataChan
		if !ok {
			return connectionClosedMsg{}
		}
		return dataReceivedMsg{data: data}
	}
}

const chunkSize = 16 * 1024 // 16KB is a standard WebRTC optimal chunk size

func cmdChunkAndSendManifest(p *p2p.P2pManager, path string) tea.Cmd {
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to read file stat: %w", err)}
		}

		totalChunks := int(math.Ceil(float64(info.Size()) / float64(chunkSize)))
		manifest := Manifest{
			Filename: info.Name(),
			Size:     info.Size(),
			Chunks:   totalChunks,
		}

		b, err := json.Marshal(manifest)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to marshal manifest: %w", err)}
		}

		if err := p.SafeWriteBytesToDC(b); err != nil {
			return errMsg{err: err}
		}

		return manifestSentMsg{}
	}
}

func cmdSendChunks(p *p2p.P2pManager, path string, progressChan chan float64) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(path)
		if err != nil {
			return errMsg{err: err}
		}
		defer file.Close()

		info, _ := file.Stat()
		buffer := make([]byte, chunkSize)
		var bytesSent int64

		for {
			n, err := file.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}
				return errMsg{err: err}
			}

			if err := p.SafeWriteBytesToDC(buffer[:n]); err != nil {
				return errMsg{err: err}
			}

			bytesSent += int64(n)
			progress := (float64(bytesSent) / float64(info.Size())) * 100

			select {
			case progressChan <- progress:
			default:
			}
		}

		close(progressChan)
		return transferCompleteMsg{}
	}
}

func listenForProgress(progressChan chan float64) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-progressChan
		if !ok {
			return nil
		}
		return chunkProgressMsg{percent: p}
	}
}
