package goraft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type appendEntriesPayload struct {
	// leader’s term
	Term int

	// so follower can redirect clients
	LeaderID string

	// index of log entry immediately preceding new ones
	PrevLogIndex int

	// term of prevLogIndex entry
	PrevLogTerm int

	// log entries to store (empty for heartbeat;
	// may send more than one for efficiency)
	Entries []*logEntry

	// leader’s commitIndex
	LeaderCommit int
}

type appendEntriesResponse struct {
	// currentTerm, for leader to update itself
	Term int

	// true if follower contained entry matching
	// prevLogIndex and prevLogTerm
	Success bool
}

type requestVotePayload struct {
	// candidate’s term
	Term int

	// candidate requesting vote
	CandidateID string

	// index of candidate’s last log entry
	LastLogIndex int

	// term of candidate’s last log entry
	LastLogTerm int
}

type requestVoteResponse struct {
	// currentTerm, for candidate to update itself
	Term int

	// true means candidate received vote
	VoteGranted bool
}

type clientApplyPayload struct {
	Command string
}

type clientApplyResponse struct {
	Success bool
	Leader  string
}

func appendEntries(uri url.URL, payload *appendEntriesPayload) (*appendEntriesResponse, error) {
	payloadStr, _ := json.Marshal(payload)
	toSend := bytes.NewBuffer(payloadStr)

	client := &http.Client{
		Timeout: heartbeatInterval * time.Millisecond,
	}

	uri.Path = "/appendEntries"
	//log.Printf("Sending request to %s\n", uri.String())
	r, _ := http.NewRequest("POST", uri.String(), toSend)

	res, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to make append entries request: %s",
			err.Error())
	}
	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to read response in append entries request: %s", err)
	}

	appendEntriesRes := &appendEntriesResponse{}
	if err := json.Unmarshal(content, appendEntriesRes); err != nil {
		return nil, fmt.Errorf(
			"Failed to unmarshal append entries response '%s' (%s) with error %s",
			content, res.Status, err.Error())
	}

	return appendEntriesRes, nil
}

func requestVote(uri url.URL, payload *requestVotePayload) (*requestVoteResponse, error) {
	payloadStr, _ := json.Marshal(payload)
	toSend := bytes.NewBuffer(payloadStr)

	client := &http.Client{
		Timeout: heartbeatInterval * time.Millisecond,
	}

	uri.Path = "/requestVote"
	//log.Printf("Sending request to %s\n", uri.String())
	r, _ := http.NewRequest("POST", uri.String(), toSend)

	res, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to make request vote request: %s",
			err.Error())
	}
	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to read response in request vote request: %s", err)
	}

	requestVoteRes := &requestVoteResponse{}
	if err := json.Unmarshal(content, requestVoteRes); err != nil {
		return nil, fmt.Errorf(
			"Failed to unmarshal request vote response '%s' (%s) with error %s",
			content, res.Status, err.Error())
	}

	return requestVoteRes, nil
}
