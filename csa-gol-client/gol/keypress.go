package gol

type keypressChannels struct {
	keyPresses <-chan rune       // An in-channel to receive keypresses
	signals chan<- signals // An out-channel to send key instructions
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type signals uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	save signals = iota
	quit
	pause
)

func startKeypress(p Params, c keypressChannels) {
	for {
		// Always wait for a keypress
		key := <-c.keyPresses

		// Figure out what the key means
		switch key {
		case 's':
			c.signals <- save
		case 'q':
			c.signals <- quit
		case 'p':
			c.signals <- pause
		}
	}
}
