package goraft

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type serverMethods struct {
	appendEntries func(*appendEntriesPayload) (*appendEntriesResponse, error)
	requestVote   func(*requestVotePayload) (*requestVoteResponse, error)
}

type server struct {
	methods *serverMethods
	port    int
	log     *log.Logger
	stopped bool
}

func newServer(logger *log.Logger, port int, methods *serverMethods) *server {
	return &server{
		log:     logger,
		methods: methods,
		port:    port,
		stopped: false,
	}
}

func (s *server) run() {
	portStr := ":" + strconv.Itoa(s.port)
	s.log.Printf("Running server on %s\n", portStr)
	http.ListenAndServe(portStr, s)
}

func (s *server) stop() {
	s.log.Println("Stopping server...")
	s.stopped = true
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.stopped {
		notFound(w, r)
		return
	}

	switch r.URL.Path {
	case "/appendEntries":
		onlyPOST(s.appendEntriesHandler)(w, r)

	case "/requestVote":
		onlyPOST(s.requestVoteHandler)(w, r)

	default:
		notFound(w, r)
	}
}

func (s *server) appendEntriesHandler(w http.ResponseWriter,
	r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	payload := &appendEntriesPayload{}
	err := decoder.Decode(&payload)
	if err != nil {
		s.log.Printf("Failed server append entries %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	s.log.Printf("Received append entries request with payload: %+v\n", payload)

	res, err := s.methods.appendEntries(payload)
	if err != nil {
		s.log.Printf("Failed server append entries %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.log.Printf("Replying to append entries with: %+v\n", res)

	json.NewEncoder(w).Encode(res)
}

func (s *server) requestVoteHandler(w http.ResponseWriter,
	r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	payload := &requestVotePayload{}
	err := decoder.Decode(&payload)
	if err != nil {
		s.log.Printf("Failed server request vote %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	s.log.Printf("Received request vote request with payload: %+v\n", payload)

	res, err := s.methods.requestVote(payload)
	if err != nil {
		s.log.Printf("Failed server request vote %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.log.Printf("Replying to request vote with: %+v\n", res)

	json.NewEncoder(w).Encode(res)
}

func onlyPOST(fn func(http.ResponseWriter, *http.Request)) func(
	http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			fn(w, r)

		default:
			notFound(w, r)
		}
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}
