package stubs

type BoardMsg struct {
	Height   int
	Width    int
	MaxTurns int

	Board [][]bool
}

// type UpdateMessage struct {
// 	CompletedTurns  int
// 	State           int
// 	AliveCellsCount int
// 	AliveCells      []util.Cell
// 	Board           [][]bool
// }

// // StateChange is an Event notifying the user about the change of state of execution.
// // This Event should be sent every time the execution is paused, resumed or quit.
// type StateChangeMsg struct { // implements Event
// 	CompletedTurns int
// 	NewState       int
// }

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

// State represents a change in the state of execution.
type State int

const (
	Paused State = iota
	Executing
	Quitting
)

// String methods allow the different types of Events and States to be printed.

func (state State) String() string {
	switch state {
	case Paused:
		return "Paused"
	case Executing:
		return "Executing"
	case Quitting:
		return "Quitting"
	default:
		return "Incorrect State"
	}
}

var ServerStartGame = "Server.StartGame"
var ServerRegisterKeypress = "Server.RegisterKeypress"
var ServerConnectWorker = "Server.ConnectWorker"

var ClientGameStateChange = "Client.GameStateChange"
var ClientTurnComplete = "Client.TurnComplete"
var ClientFinalTurnComplete = "Client.FinalTurnComplete"
var ClientSaveBoard = "Client.SaveBoard"
var ClientReportAliveCells = "Client.ReportAliveCells"

var WorkerDoTurn = "Worker.DoTurn"

type ServerResponse struct {
	Success bool
	Message string
}

type ClientResponse struct {
	Success bool
}

type StartGameRequest struct {
	ClientAddress string

	Height   int
	Width    int
	MaxTurns int

	Board [][]bool
}

type KeypressRequest struct {
	// Key rune
	Key Signals
}

type WorkerConnectRequest struct {
	WorkerAddress string
}

type StateChangeReport struct {
	Previous       State
	New            State
	CompletedTurns int
}

type TurnCompleteReport struct {
	CompletedTurns int
}

type SaveBoardRequest struct {
	CompletedTurns int

	Height int
	Width  int
	Board  [][]bool
}

type AliveCellsReport struct {
	CompletedTurns int
	NumAlive       int
}

type DoTurnRequest struct {
	Board     [][]bool
	FragStart int
	FragEnd   int
}
type DoTurnResponse struct {
	Frag Fragment
}

type Empty struct{}
