package stubs

// Fragment stores a section of cells in the board
// StartRow points to the row in the main grid where this section starts
// EndRow points to the next row in the main grid after this section ends (like an exclusive upper bound)
type Fragment struct {
	StartRow int
	EndRow   int
	Cells    [][]bool
}

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

//    RPC STRINGS

// Server RPC strings
var ServerStartGame = "Server.StartGame"
var ServerRegisterKeypress = "Server.RegisterKeypress"
var ServerConnectWorker = "Server.ConnectWorker"

// Client RPC strings
var ClientGameStateChange = "Client.GameStateChange"
var ClientTurnComplete = "Client.TurnComplete"
var ClientFinalTurnComplete = "Client.FinalTurnComplete"
var ClientSaveBoard = "Client.SaveBoard"
var ClientReportAliveCells = "Client.ReportAliveCells"

// Worker RPC strings
var WorkerDoTurn = "Worker.DoTurn"

// ServerResponse contains a result from a standard server RPC call
// Success indicates if the call executed its desired function
// Message contains any additional information
type ServerResponse struct {
	Success bool
	Message string
}

// StartGameRequest contains all data required for a client to connect to a server
// and start a game
// This will send the address of the client, along with information about the board
// and the starting board state
type StartGameRequest struct {
	ClientAddress string

	Height   int
	Width    int
	MaxTurns int

	Board [][]bool
}

// KeypressRequest is used to send a keypress from a client to be handled at the server
type KeypressRequest struct {
	Key rune
}

// WorkerConnectRequest is passed by a worker which wishes to connect to the server
// This contains the address of the worker so the server can establish a connection
type WorkerConnectRequest struct {
	WorkerAddress string
}

// StateChangeReport is passed to the client to inform them of changes to game state 
type StateChangeReport struct {
	Previous       State
	New            State
	CompletedTurns int
}

// TurnCompleteReport is passed to the client every time a turn is completed
type TurnCompleteReport struct {
	CompletedTurns int
}

// SaveBoardRequest is passed to the client to ask them to save the board 
type SaveBoardRequest struct {
	CompletedTurns int

	Height int
	Width  int
	Board  [][]bool
}

// AliveCellsReport is passed to the client every 2 seconds to tell them how many
// cells are alive
type AliveCellsReport struct {
	CompletedTurns int
	NumAlive       int
}

// DoTurnRequest is passed to workers to ask them to calculate the next turn
// It sends the whole board along with fragment pointers for their portion to calculate
type DoTurnRequest struct {
	Board     [][]bool
	FragStart int
	FragEnd   int
}

// DoTurnResponse is returned by workers to the server containing a fragment of the new board
type DoTurnResponse struct {
	Frag Fragment
}

// Empty is used when there is no information for an RPC function to return 
type Empty struct{}
