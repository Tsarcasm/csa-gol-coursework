package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns         int
	Threads       int
	ImageWidth    int
	ImageHeight   int
	ServerAddress string
	Port          string
	OurIP         string
	VisualUpdates bool
	ResumeGame    bool
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
	// Get our public IP address
	if p.OurIP == "" {
		p.OurIP = util.GetPublicIP()
	}
	println("Our IP Address: ", p.OurIP)
	// If params doesn't have defaults for network connections, set them
	if p.Port == "" {
		p.Port = "8050"
	}
	if p.ServerAddress == "" {
		// Default to the flexible IP address of the AWS server instance
		p.ServerAddress = "54.156.128.45:8030"
	}

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioImageInput := make(chan uint8)
	ioImageOutput := make(chan uint8)

	controllerChannels := controllerChannels{
		events,
		ioCommand,
		ioIdle,
		ioFilename,
		ioImageInput,
		ioImageOutput,
		keyPresses,
	}
	go controller(p, controllerChannels)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioImageOutput,
		input:    ioImageInput,
	}
	go startIo(p, ioChannels)

}
