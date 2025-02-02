package sdl

import (
	"fmt"
	// "reflect"

	"github.com/veandco/go-sdl2/sdl"
	"uk.ac.bris.cs/gameoflife/gol"
)

func Start(p gol.Params, events <-chan gol.Event, keyPresses chan<- rune) {
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))

sdlLoop:
	for {
		event := w.PollEvent()
		if event != nil {
			switch e := event.(type) {
			case *sdl.KeyboardEvent:
				switch e.Keysym.Sym {

				case sdl.K_p:
					keyPresses <- 'p'
				case sdl.K_s:
					keyPresses <- 's'
				case sdl.K_q:
					keyPresses <- 'q'
				case sdl.K_k:
					keyPresses <- 'k'
				case sdl.K_r:
					keyPresses <- 'r'
				}
			}
		}
		select {
		case event, ok := <-events:
			if !ok {
				println("Break!")
				w.Destroy()
				break sdlLoop
			}
			switch e := event.(type) {
			case gol.CellFlipped:
				w.FlipPixel(e.Cell.X, e.Cell.Y)
			case gol.TurnComplete:
				w.RenderFrame()
			default:
				if len(event.String()) > 0 {
					fmt.Printf("Completed Turns %-8v%v\n", event.GetCompletedTurns(), event)
				}
			}
		default:
			break
		}
	}

}
