package nodestbl

import (
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"sync"
	"time"
)

// In this file are defined types and methods to manage nodestbl information
// specific for the Node Margin Strategy

//////////////////// PUBLIC STRUCT TYPES ////////////////////

// Load represents information about the requests load on a certain node
type Load struct {
	RateHighUsage 	float64 // Rate for functions in "high usage" group
	RateLowUsage 	float64 // Rate for functions in "low usage" group
	RateMediumUsage float64 // Rate for functions in "medium usage" group
}

// EntryNMS represents an entry with data of a certain node for Node Margin Strategy
type EntryNMS struct {
	// Node's ID
	ID string
	// Time of the last message received from the node
	TAlive time.Time
	// Node's HAProxy host and port
	HAProxyHost string
	HAProxyPort uint
	// Specifies if the node has at least a function in common
	CommonNeighbour bool
	// Type of the node (light, mid or heavy)
	NodeType int
	// Node's functions
	Funcs []string
	// Node's load
	Load Load
	// Node's margin
	Margin float64
}

// TableNMS is actually a list of instances of the EntryNMS struct, which can be
// accessed with concurrency-safe methods only
type TableNMS struct {
	mutex   *sync.Mutex
	EntryValidity time.Duration
	// The list of the entries. The key is the node ID
	entries map[string]*EntryNMS
}

//////////////////// PRIVATE METHODS ////////////////////

// InitTable initializes a TableNMS's fields if they are empty
func (tbl *TableNMS) initTable() {
	logger := logging.Logger()

	if tbl.entries == nil {
		tbl.entries = map[string]*EntryNMS{}
		logger.Debug("Initialized table entries")
	}

	if tbl.mutex == nil {
		tbl.mutex = &sync.Mutex{}
		logger.Debug("Initialized table mutex")
	}
}

// isExpired returns true if the EntryNMS is expired, according to
// tbl.EntryValidity and entry.TAlive
func (tbl *TableNMS) isExpired(entry *EntryNMS) bool {
	return entry.TAlive.Add(tbl.EntryValidity).Before(time.Now())
}

//////////////////// PUBLIC METHODS ////////////////////

// NewTableNMS creates and initializes a new TableNMS returning a pointer to the table
func NewTableNMS(entryValidity time.Duration) *TableNMS {
	tbl := &TableNMS{
		EntryValidity: entryValidity,
	}
	tbl.initTable()

	return tbl
}

// SafeExec lets you execute a generic operation on the TableNMS entries atomically
// (in a critical section). You can safely pass the entries map by value to
// function, instead of a pointer, because a map is itself a pointer type
func (tbl *TableNMS) SafeExec(function func(entries map[string]*EntryNMS) error) error {
	logger := logging.Logger()
	tbl.mutex.Lock()
	defer tbl.mutex.Unlock()

	tbl.initTable()

	// Remove expired entries before doing anything
	logger.Debug("Removing expired table entries")
	for nodeID, entry := range tbl.entries {
		if tbl.isExpired(entry) {
			delete(tbl.entries, nodeID)
			logger.Debugf("Entry %s for node %s is expired and has been deleted", entry.ID, nodeID)
		}
	}

	return function(tbl.entries)
}
