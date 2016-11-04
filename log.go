package goraft

type logEntry struct {
	// Term when entry was received by leader
	Term int

	// Command for state machine
	Command string

	// Index is a position in the log
	Index int
}
