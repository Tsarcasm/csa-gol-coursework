package gol

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
