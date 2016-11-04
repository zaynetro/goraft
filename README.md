# Go implementation of Raft consensus algorithm

Raft description: https://raft.github.io/

**Work in progress**

## Should be fixed

* [x] Log indices start at -1
* [ ] Validate all struct fields in the tests
* [ ] Maintain mapping between nodes and last applied log entry
* [ ] Retry appendEntries RPC indefinitely
* [ ] Maybe call raft methods from server with channels
      (this is not straighforward and adds a lot of complexity)

## Implemented features

* [x] Leader election
* [ ] Log replication
* [ ] Consistent state machines
    * [ ] If leader executed a command and crashed, then client retries the same
      command with a new leader. This way state will be changed two times.
      Will be nice to send a unique serial number with all the commands.
      (See section 8: "Client integration" for more)
* [ ] Live configuration changes (e.g. number of nodes)
* [ ] State restoration (transfer snapshots)
* [ ] Log compaction
