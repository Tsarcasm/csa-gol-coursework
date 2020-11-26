package gol

import (
"uk.ac.bris.cs/gameoflife/stubs"
	
)
// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioImageInput := make(chan uint8)
	ioImageOutput := make(chan uint8)

	signals := make(chan stubs.Signals)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		ioFilename,
		ioImageInput,
		ioImageOutput,
		signals,
	}
	go distributor(p, distributorChannels)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioImageOutput,
		input:    ioImageInput,
	}
	go startIo(p, ioChannels)

	keypressChannels := keypressChannels{
		keyPresses,
		signals,
	}

	go startKeypress(p, keypressChannels)

}
