package goraft

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
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

func (c cluster) find(serverType serverType) []*raft {
	rafts := []*raft{}
	for _, r := range c.rafts {
		if r.state.serverType == serverType {
			rafts = append(rafts, r)
		}
	}
	return rafts
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

func clientApply(uri url.URL, command string) (*clientApplyResponse, error) {
	payloadStr, _ := json.Marshal(&clientApplyPayload{
		Command: command,
	})
	toSend := bytes.NewBuffer(payloadStr)

	client := &http.Client{
		Timeout: 700 * time.Millisecond,
	}

	uri.Path = "/clientApply"
	r, _ := http.NewRequest("POST", uri.String(), toSend)

	res, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	clientApplyRes := &clientApplyResponse{}
	err = json.Unmarshal(content, clientApplyRes)
	return clientApplyRes, err
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

	assert.Equal(1, c.nodesNum(leader), "One node should be a leader")
	assert.Equal(n-1, c.nodesNum(follower), "Other nodes should be followers")

	for i := 1; i <= 2; i++ {
		c.removeNode(leader)
		<-time.After(400 * time.Millisecond)

		assert.Equal(1, c.nodesNum(leader), "New leader should be selected")
		assert.Equal(n-i-1, c.nodesNum(follower), "Other nodes should be followers")
	}

	c.removeNode(leader)
	<-time.After(400 * time.Millisecond)
	assert.True(c.nodesNum(candidate) > 0, "There is at least one candidate")

	c.removeAll()
}

func TestLogReplication3Nodes(t *testing.T) {
	assert := assert.New(t)

	n := 3
	c := newCluster(n, 3020)
	c.runAll()
	<-time.After(400 * time.Millisecond)

	command := "hello world"
	leaders := c.find(leader)

	{
		// Add log entry
		assert.Equal(1, len(leaders), "One node should be a leader")

		log.Println("======================================================")
		log.Println("======================================================")
		log.Println("======================================================")
		clientApplyRes, err := clientApply(*leaders[0].current.uri, command)
		assert.Nil(err, "Client apply request should not fail")
		assert.True(clientApplyRes.Success,
			"Clint apply request should be successfull")
	}

	<-time.After(100 * time.Millisecond)

	{
		// Verify log entry is saved on followers
		followers := c.find(follower)
		assert.Equal(n-1, len(followers), "Other nodes should be followers")

		for _, f := range followers {
			assert.Equal(1, len(f.state.log), "There should one entry in log")
			assert.Equal(command, f.state.log[0].Command, "Command is the same")
		}

		// Submit log entry to follower
		clientApplyRes, err := clientApply(*followers[0].current.uri, command)
		assert.Nil(err, "Client apply request should not fail")
		assert.False(clientApplyRes.Success,
			"Clint apply request should be unsuccessfull")
		assert.NotNil(clientApplyRes.Leader,
			"Clint apply request should return leader")
		assert.Equal(clientApplyRes.Leader, leaders[0].current.uri.String(),
			"Clint apply request should return correct leader")
	}

	c.removeAll()
}
