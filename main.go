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

	m := tui.Model{}

	// 1. Initialize your p2pManager and its channels
	manager := &p2p.P2pManager{
		StatusChan:  make(chan string, 10),
		ErrorChan:   make(chan error, 10),
		MessageChan: make(chan p2p.EventMessage, 10),
		DataChan:    make(chan []byte, 100),
		ID:          m.GenerateTOTP(), // Initialize ID here
	}

	p := tea.NewProgram(m.InitialModel(manager))

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
