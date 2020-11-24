package gol

type keypressChannels struct {
	keyPresses <-chan rune       // An in-channel to receive keypresses
	keyCommand chan<- keyCommand // An out-channel to send key instructions
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type keyCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	save keyCommand = iota
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
			c.keyCommand <- save
		case 'q':
			c.keyCommand <- quit
		case 'p':
			c.keyCommand <- pause
		}
	}
}
