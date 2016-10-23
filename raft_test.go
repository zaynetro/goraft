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

type cluster struct {
	rafts []*raft
}

func newCluster(n, portOffset int) *cluster {
	nodes := getNodes(n, portOffset)
	rafts := []*raft{}

	for _, n := range nodes {
		rafts = append(rafts, newRaft(n.id, nodes...))
	}

	return &cluster{
		rafts: rafts,
	}
}

func (c *cluster) runAll() {
	for _, r := range c.rafts {
		log.Println("Starting node", r.current.colored())
		go r.run()
	}
}

func (c cluster) nodesNum(serverType serverType) int {
	nodes := 0
	for _, r := range c.rafts {
		if r.state.serverType == serverType {
			nodes++
		}
	}
	return nodes
}

func (c *cluster) removeNode(serverType serverType) {
	removeIndex := -1
	for i, r := range c.rafts {
		if r.state.serverType == serverType {
			log.Printf("Exiting node %s\n", r.current.colored())
			r.exit()
			removeIndex = i
		}
	}

	if removeIndex >= 0 {
		c.rafts[removeIndex] = c.rafts[len(c.rafts)-1]
		c.rafts[len(c.rafts)-1] = nil
		c.rafts = c.rafts[:len(c.rafts)-1]
	}
}

func (c *cluster) removeAll() {
	for _, r := range c.rafts {
		log.Printf("Exiting node %s\n", r.current.colored())
		r.exit()
	}
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

func TestMain(m *testing.M) {
	log.SetOutput(os.Stdout)
	os.Exit(m.Run())
}

func TestMasterReelection5Nodes(t *testing.T) {
	assert := assert.New(t)

	n := 5
	c := newCluster(n, 3010)
	c.runAll()
	<-time.After(400 * time.Millisecond)

	assert.Equal(c.nodesNum(leader), 1, "One node should be a leader")
	assert.Equal(c.nodesNum(follower), n-1, "Other nodes should be followers")

	for i := 1; i <= 2; i++ {
		c.removeNode(leader)
		<-time.After(400 * time.Millisecond)

		assert.Equal(c.nodesNum(leader), 1, "New leader should be selected")
		assert.Equal(c.nodesNum(follower), n-i-1, "Other nodes should be followers")
	}

	c.removeNode(leader)
	<-time.After(400 * time.Millisecond)
	assert.True(c.nodesNum(candidate) > 0, "There is at least one candidate")

	c.removeAll()
}
