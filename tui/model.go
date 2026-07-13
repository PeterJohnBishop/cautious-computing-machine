package tui

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/peterjohnbishop/cautious-computing-machine/p2p"
)

type Model struct {
	p2p        *p2p.P2pManager
	role       int
	focusIndex int
	pathInput  textinput.Model
	totpInput  textinput.Model
	totpSecret string
	logs       []string
	help       help.Model

	progressChan     chan float64
	manifestReceived bool
	manifest         Manifest
	receivedChunks   int
	fileWriter       *os.File
}

func GenerateTOTP() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%06d", r.Intn(1000000))
}

func InitialModel(p *p2p.P2pManager) Model {
	path := textinput.New()
	path.Placeholder = "file path"
	path.CharLimit = 150
	path.SetWidth(40)

	totp := textinput.New()
	totp.Placeholder = "enter 6-digit TOTP code"
	totp.CharLimit = 6
	totp.SetWidth(20)

	if p.ID == "" {
		p.ID = GenerateTOTP()
	}

	return Model{
		p2p:        p,
		role:       RoleSender,
		focusIndex: FocusToggle,
		pathInput:  path,
		totpInput:  totp,
		totpSecret: p.ID,
		logs:       []string{"[System] Ready to configure connection."},
		help:       help.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		listenForStatus(m.p2p.StatusChan),
		listenForErrors(m.p2p.ErrorChan),
	)
}
