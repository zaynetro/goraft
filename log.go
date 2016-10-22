package goraft

type logEntry struct {
	// term when entry was received by leader
	term int64

	// command for state machine
	command string
}
