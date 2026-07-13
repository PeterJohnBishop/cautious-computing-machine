package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/joho/godotenv"
	"github.com/peterjohnbishop/cautious-computing-machine/p2p"
	"github.com/peterjohnbishop/cautious-computing-machine/tui"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

	manager := &p2p.P2pManager{
		StatusChan:  make(chan string, 100),
		ErrorChan:   make(chan error, 100),
		MessageChan: make(chan p2p.EventMessage, 100),
		DataChan:    make(chan []byte, 1024),
		ID:          tui.GenerateTOTP(),
	}

	p := tea.NewProgram(tui.InitialModel(manager))

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
