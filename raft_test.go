package goraft

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var colors = []string{
	string([]byte{27, 91, 57, 55, 59, 52, 50, 109}), // green
	string([]byte{27, 91, 57, 55, 59, 52, 51, 109}), // yellow
	string([]byte{27, 91, 57, 55, 59, 52, 52, 109}), // blue
	string([]byte{27, 91, 57, 55, 59, 52, 53, 109}), // magenta
	string([]byte{27, 91, 57, 55, 59, 52, 54, 109}), // cyan
	string([]byte{27, 91, 57, 48, 59, 52, 55, 109}), // white
	//string([]byte{27, 91, 57, 55, 59, 52, 49, 109}), // red
}

func TestMasterElectionRaft(t *testing.T) {
	log.SetOutput(os.Stdout)
	assert := assert.New(t)

	// Start 3 nodes
	// Wait till all started
	// One node should be a leader
	// Others should be followers
	// Kill first node
	// One of the remaining ones should become a candidate
	// Done

	n := 3

	nodes := getNodes(n, 3000)
	rafts := []*raft{}

	log.Printf("Nodes: %+v\n", nodes)

	for _, n := range nodes {
		log.Printf("Starting node %s\n", n.colored())
		r := newRaft(n.id, nodes...)
		rafts = append(rafts, r)
		go r.run()
	}

	<-time.After(500 * time.Millisecond)

	assert.Equal(leadersNum(serverTypes(rafts...)...), 1,
		"One node should be a leader, others followers")

	// Terminate leader node
	{
		removeIndex := -1
		for i, r := range rafts {
			if r.state.serverType == leader {
				log.Printf("Exiting node %s\n", nodes[i].colored())
				r.exit()
				removeIndex = i
			}
		}

		if removeIndex >= 0 {
			rafts[removeIndex] = rafts[len(rafts)-1]
			rafts[len(rafts)-1] = nil
			rafts = rafts[:len(rafts)-1]
		}
	}

	<-time.After(500 * time.Millisecond)

	assert.Condition(func() bool {
		first, second := rafts[0].state.serverType, rafts[1].state.serverType
		return first == candidate || second == candidate
	}, "One of the running nodes should be a candidate")

	// Terminate others
	rafts[0].exit()
	rafts[1].exit()
}

func getNodes(n int, portOffset int) []*node {
	nodes := []*node{}
	for i := 0; i < n; i++ {
		port := portOffset + i
		server, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
		nodes = append(nodes, &node{
			uri:   server,
			id:    "node-" + strconv.Itoa(port),
			color: getColor(i),
			port:  port,
		})
	}
	return nodes
}

func getColor(i int) string {
	return colors[i]
}

func serverTypes(rafts ...*raft) []serverType {
	types := []serverType{}
	for _, r := range rafts {
		types = append(types, r.state.serverType)
	}
	return types
}

func leadersNum(serverTypes ...serverType) int {
	leaders := 0
	for _, serverType := range serverTypes {
		if serverType == leader {
			leaders++
		}
	}
	return leaders
}
