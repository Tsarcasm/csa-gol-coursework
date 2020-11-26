package gol
import (	
	"uk.ac.bris.cs/gameoflife/stubs"
)

type keypressChannels struct {
	keyPresses <-chan rune       // An in-channel to receive keypresses
	signals chan<- stubs.Signals // An out-channel to send key instructions
}



func startKeypress(p Params, c keypressChannels) {
	for {
		// Always wait for a keypress
		key := <-c.keyPresses

		// Figure out what the key means
		switch key {
		case 's':
			c.signals <- stubs.Save
		case 'q':
			c.signals <- stubs.Quit
		case 'p':
			c.signals <- stubs.Pause
		}
	}
}
