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
	currentTerm int

	// candidateId that received vote in current term (or null if none)
	votedFor *string

	// log entries; each entry contains command for state machine and
	// term when entry was received by leader (first index is 1)
	log []*logEntry

	// index of highest log entry known to be committed
	// (initialized to 0, increases monotonically)
	commitIndex int

	// index of hightest log entry applied to a state machine
	// (initialized to 0, increases monotonically)
	lastApplied int

	// The following are for leader nodes only
	// Reset after election

	// for each server, index of the next log entry to send to that server
	// (initialized to leader last log index + 1)
	nextIndex map[string]int

	// for each server, index of highest log entry known to be
	// replicated on server (initialized to 0, increases monotonically)
	matchIndex map[string]int
}

func newState() *state {
	return &state{
		serverType:  follower,
		currentTerm: 0,
		votedFor:    nil,
		log:         []*logEntry{},
		commitIndex: -1,
		lastApplied: -1,
		// Node is not a leader, don't init the following fields yet
		nextIndex:  nil,
		matchIndex: nil,
	}
}

func (s state) lastLogTerm() int {
	if len(s.log) == 0 {
		return 0
	}

	return s.log[len(s.log)-1].Term
}

func (s state) logEntryTerm(index int) int {
	if index > len(s.log) || index < 0 {
		return 0
	}

	return s.log[index].Term
}

func (s *state) becomeLeader(nodes map[string]*node) {
	s.serverType = leader
	s.votedFor = nil
	s.nextIndex = make(map[string]int)
	s.matchIndex = make(map[string]int)

	for _, node := range nodes {
		s.nextIndex[node.id] = s.lastApplied + 1
		s.matchIndex[node.id] = -1
	}
}
