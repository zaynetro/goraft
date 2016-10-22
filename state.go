package goraft

type serverType int

const (
	follower serverType = iota + 1
	candidate
	leader
)

type state struct {
	// follower, candiate or leader type
	serverType serverType

	// Latest term server has seen n (initialized to 0
	// on first boot, increases monotonically)
	currentTerm int64

	// candidateId that received vote in current term (or null if none)
	votedFor *string

	// log entries; each entry contains command for state machine and
	// term when entry was received by leader (first index is 1)
	log []*logEntry

	// index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex int64

	// index of hightest log entry applied to a state machine
	// (initialized to 0, increases monotonically)
	lastApplied int64

	// The following are for leader nodes only
	// Reset after election

	// for each server, index of the next log entry to send to that server
	// (initialized to leader last log index + 1)
	nextIndex map[string]int64

	// for each server, index of highest log entry known to be
	// replicated on server (initialized to 0, increases monotonically)
	matchIndex map[string]int64
}

func newState() *state {
	return &state{
		serverType:  follower,
		currentTerm: 0,
		votedFor:    nil,
		log:         []*logEntry{},
		commitIndex: 0,
		lastApplied: 0,
		// Node is not a leader, don't init the following fields yet
		nextIndex:  nil,
		matchIndex: nil,
	}
}

func (s state) lastLogIndex() int {
	if len(s.log) == 0 {
		return 0
	}

	return len(s.log) - 1
}

func (s state) lastLogTerm() int64 {
	if len(s.log) == 0 {
		return 0
	}

	return s.log[len(s.log)-1].term
}
