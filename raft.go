package goraft

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const heartbeatInterval = 150 * time.Millisecond

var electionTimeoutRangeMillis = []int{150, 300}

type raft struct {
	nodes   []*node
	current *node
	state   *state
	server  *server
	ch      *raftChannels
	log     *log.Logger
	stopped bool
}

type raftChannels struct {
	appendEntries chan struct{}
	requestVote   chan struct{}
	exit          chan struct{}
}

type node struct {
	uri   *url.URL
	id    string
	color string
	port  int
}

func (n node) colored() string {
	reset := string([]byte{27, 91, 48, 109})
	return fmt.Sprintf("%s[%s]%s", n.color, n.id, reset)
}

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

func newRaft(currentID string, nodes ...*node) *raft {
	var currentNode *node
	otherNodes := []*node{}
	for _, n := range nodes {
		if n.id == currentID {
			currentNode = n
		} else {
			otherNodes = append(otherNodes, n)
		}
	}

	logFlags := log.Ldate | log.Lmicroseconds | log.Lshortfile
	prefix := currentNode.colored() + " "
	logger := log.New(os.Stdout, prefix, logFlags)

	r := &raft{
		nodes:   otherNodes,
		current: currentNode,
		state:   newState(),
		ch: &raftChannels{
			appendEntries: make(chan struct{}),
			requestVote:   make(chan struct{}),
			exit:          make(chan struct{}),
		},
		log:     logger,
		stopped: false,
	}

	r.log.Printf("Initiating raft with current node %s: %s\n",
		currentNode.colored(), currentNode.uri.String())
	for _, n := range otherNodes {
		r.log.Printf("Initiating raft with other nodes %s: %s\n", n.colored(),
			n.uri.String())
	}

	r.server = newServer(logger, currentNode.port, &serverMethods{
		appendEntries: r.appendEntries,
		requestVote:   r.requestVote,
	})

	return r
}

func (r *raft) run() {
	r.log.Println("Starting raft server...")
	go r.server.run()

	r.log.Println("Starting raft routine...")
	go r.routine()

	<-r.ch.exit
	r.log.Println("Exiting raft...")
	r.server.stop()
	r.stopped = true
}

func (r *raft) exit() {
	r.ch.exit <- struct{}{}
}

func (r *raft) appendEntries(req *appendEntriesPayload) (*appendEntriesResponse, error) {

	if r.state.serverType == leader {
		r.log.Println("Server is a leader, skipping append entries...")
		return nil, nil
	}

	r.ch.appendEntries <- struct{}{}

	// Receiver implementation:
	// 1. Reply false if term < currentTerm (§5.1)
	// 2. Reply false if log doesn’t contain an entry at prevLogIndex
	//    whose term matches prevLogTerm (§5.3)
	// 3. If an existing entry conflicts with a new one (same index
	//    but different terms), delete the existing entry and all that
	//    follow it (§5.3)
	// 4. Append any new entries not already in the log
	// 5. If leaderCommit > commitIndex, set commitIndex =
	//    min(leaderCommit, index of last new entry)

	falseReply := &appendEntriesResponse{
		Term:    r.state.currentTerm,
		Success: false,
	}

	if req.Term < r.state.currentTerm {
		return falseReply, nil
	}

	if int64(len(r.state.log)) < req.PrevLogIndex ||
		r.state.lastLogTerm() != req.PrevLogTerm {
		return falseReply, nil
	}

	if len(r.state.log) > 0 {
		r.state.log = r.state.log[:req.PrevLogIndex+1]
	}
	r.state.log = append(r.state.log, req.Entries...)

	if req.LeaderCommit > r.state.commitIndex {
		r.state.commitIndex = int64(
			math.Min(
				float64(req.LeaderCommit), float64(r.state.lastLogIndex())))
	}

	return &appendEntriesResponse{
		Term:    r.state.currentTerm,
		Success: true,
	}, nil
}

func (r *raft) requestVote(req *requestVotePayload) (*requestVoteResponse, error) {
	if r.state.serverType == leader {
		r.log.Println("Server is a leader, skipping request vote...")
		return nil, nil
	}

	r.ch.requestVote <- struct{}{}

	// Receiver implementation:
	// 1. Reply false if term < currentTerm (§5.1)
	// 2. If votedFor is null or candidateId, and candidate’s log is at
	//    least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)

	if req.Term < r.state.currentTerm {
		r.log.Printf("Originator term (%d) is smaller than current (%d)\n",
			req.Term, r.state.currentTerm)

		return &requestVoteResponse{
			Term:        r.state.currentTerm,
			VoteGranted: false,
		}, nil
	}

	if (r.state.votedFor == nil || (*r.state.votedFor) == req.CandidateID) &&
		req.LastLogIndex >= int64(len(r.state.log)) {
		return &requestVoteResponse{
			Term:        r.state.currentTerm,
			VoteGranted: true,
		}, nil
	}

	return nil, errors.New("Couldn't deny or grant a vote")
}

func (r *raft) routine() {
	for !r.stopped {
		switch r.state.serverType {
		case follower:
			// 1. Wait for append entries
			// 2. If hasn't received append entries become candidate
			// 3. Respond to request votes
			select {
			case <-r.ch.requestVote:
				// No operation here

			case <-r.ch.appendEntries:
				// Append entries are processed in a method above
				r.log.Println("Received append entries while a follower")

			case <-r.getElectionTimeoutTicker().C:
				r.log.Println("Follower hasn't received append entries within" +
					" election timeout, becoming a candidate")
				r.state.serverType = candidate
			}

		case candidate:
			// 1. Send vote requests
			// 2. If received append entries query -> cancel attempt
			select {
			case <-r.ch.requestVote:
				// No operation here

			case <-r.ch.appendEntries:
				r.log.Println("Received append entries becoming a follower")
				r.state.serverType = follower

			case voteNum := <-r.startElection():
				r.log.Printf("Received %d votes\n", voteNum)
				if voteNum > (len(r.nodes) / 2) {
					r.log.Println("Becoming a leader...")
					r.state.serverType = leader
					r.state.votedFor = nil
				} else {
					select {
					case <-r.getElectionTimeoutTicker().C:
						r.log.Println("Election timeout has passed")

					case <-r.ch.appendEntries:
						r.log.Println("Received append entries")
					}
				}
			}

		case leader:
			// 1. Send append entries
			// 2. Wait
			r.log.Println("Sending append entries to all nodes...")
			go r.sendAppendEntries()
			r.log.Println("Waiting for hearbeatInterval", heartbeatInterval)
			select {
			case <-r.ch.requestVote:
				r.log.Println("Received request vote while a leader")

			case <-time.After(heartbeatInterval):
			}
		}
	}
}

func (r *raft) startElection() chan int {
	r.state.serverType = candidate
	r.state.currentTerm++
	r.state.votedFor = &r.current.id

	voteNumRes := make(chan int)

	go func() {
		var voteNum int32 = 1 // Vote for ourselves
		wg := sync.WaitGroup{}

		req := &requestVotePayload{
			Term:         r.state.currentTerm,
			CandidateID:  r.current.id,
			LastLogIndex: int64(r.state.lastLogIndex()),
			LastLogTerm:  r.state.lastLogTerm(),
		}

		for _, n := range r.nodes {
			wg.Add(1)
			go func(n *node) {
				defer wg.Done()

				r.log.Printf("Sending request vote %s: %+v\n", n.colored(), req)
				res, err := requestVote(*n.uri, req)
				if err != nil {
					r.log.Printf("Failed to request vote %s: %s\n", n.colored(), err)
					return
				}

				r.log.Printf("Request vote reply %s: %+v\n", n.colored(), res)
				if res.VoteGranted {
					atomic.AddInt32(&voteNum, 1)
				}

				if res.Term > r.state.currentTerm {
					// Revert to follower
					r.state.currentTerm = res.Term
					r.state.serverType = follower
					atomic.AddInt32(&voteNum, int32(-len(r.nodes)))
				}

				r.state.votedFor = nil
			}(n)
		}

		wg.Wait()
		voteNumRes <- int(voteNum)
	}()

	return voteNumRes
}

func (r *raft) sendAppendEntries() {
	wg := sync.WaitGroup{}

	req := &appendEntriesPayload{
		Term:         r.state.currentTerm,
		LeaderID:     r.current.id,
		PrevLogIndex: int64(r.state.lastLogIndex()),
		PrevLogTerm:  r.state.lastLogTerm(),
		Entries:      []*logEntry{},
		LeaderCommit: r.state.commitIndex,
	}

	for _, n := range r.nodes {
		wg.Add(1)
		go func(n *node) {
			defer wg.Done()

			r.log.Printf("Sending append entries %s: %+v\n", n.colored(), req)
			res, err := appendEntries(*n.uri, req)
			if err != nil {
				r.log.Printf("Failed to send append entries request %s %s\n",
					n.colored(), err)
				return
			}

			r.log.Printf("Append entries request response %s: %+v\n",
				n.colored(), res)

			if !res.Success {
				// TODO: act
			}

			if res.Term > r.state.currentTerm {
				// Revert to follower
				r.state.currentTerm = res.Term
				r.state.serverType = follower
			}
		}(n)
	}

	wg.Wait()
}

func (r raft) getElectionTimeoutTicker() *time.Ticker {
	begin, end := electionTimeoutRangeMillis[0], electionTimeoutRangeMillis[1]
	timeout := rand.Intn(end-begin+1) + begin

	r.log.Printf("Setting election timeout to %d ms\n", timeout)
	return time.NewTicker(time.Duration(timeout) * time.Millisecond)
}
