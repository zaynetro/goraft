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
	Term int64 `json:"term"`

	// so follower can redirect clients
	LeaderID string `json:"leaderID"`

	// index of log entry immediately preceding new ones
	PrevLogIndex int64 `json:"prevLogIndex"`

	// term of prevLogIndex entry
	PrevLogTerm int64 `json:"prevLogTerm"`

	// log entries to store (empty for heartbeat;
	// may send more than one for efficiency)
	Entries []*logEntry `json:"entries"`

	// leader’s commitIndex
	LeaderCommit int64 `json:"leaderCommit"`
}

type appendEntriesResponse struct {
	// currentTerm, for leader to update itself
	Term int64 `json:"term"`

	// true if follower contained entry matching
	// prevLogIndex and prevLogTerm
	Success bool `json:"success"`
}

type requestVotePayload struct {
	// candidate’s term
	Term int64 `json:"term"`

	// candidate requesting vote
	CandidateID string `json:"candidateID"`

	// index of candidate’s last log entry
	LastLogIndex int64 `json:"lastLogIndex"`

	// term of candidate’s last log entry
	LastLogTerm int64 `json:"lastLogTerm"`
}

type requestVoteResponse struct {
	// currentTerm, for candidate to update itself
	Term int64 `json:"term"`

	// true means candidate received vote
	VoteGranted bool `json:"voteGranted"`
}

func appendEntries(uri url.URL, payload *appendEntriesPayload) (*appendEntriesResponse, error) {
	payloadStr, _ := json.Marshal(payload)
	toSend := bytes.NewBuffer(payloadStr)

	client := &http.Client{
		Timeout: 2 * time.Second,
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
		Timeout: 2 * time.Second,
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
