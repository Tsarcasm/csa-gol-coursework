package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

type BoardMsg struct {
	Height   int
	Width    int
	MaxTurns int

	Board [][]bool
}

type UpdateMessage struct {
	CompletedTurns  int
	State           int
	AliveCellsCount int
	AliveCells      []util.Cell
	Board           [][]bool
}

// StateChange is an Event notifying the user about the change of state of execution.
// This Event should be sent every time the execution is paused, resumed or quit.
type StateChangeMsg struct { // implements Event
	CompletedTurns int
	NewState       int
}

type Fragment struct {
	StartRow int
	EndRow   int
	Cells    [][]bool
}

// A signal represents an instruction to change game state
type Signals uint8

const (
	Save Signals = iota
	Quit
	Pause
)
