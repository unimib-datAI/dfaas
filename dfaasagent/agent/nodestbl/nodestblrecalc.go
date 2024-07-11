package nodestbl

import (
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"sync"
	"time"
)

// In this file are defined types and methods to manage nodestbl information
// specific for the Recalc Strategy

//////////////////// PUBLIC STRUCT TYPES ////////////////////

// FuncData represents information about a specific function (the name is not
// saved in the struct) running on a specific node, and the limits we put on it
type FuncData struct {
	// Max req/s for this function FROM this node (we decide it)
	LimitIn float64
	// Max req/s for this function TO this node (the node itself tells us)
	LimitOut float64
	// Weight of this node for this function (we decide it)
	NodeWeight uint
}

// EntryRecalc represents an entry with data of a certain node for Recalc Strategy
type EntryRecalc struct {
	// Node's ID
	ID string
	// Time of the last message received from the node
	TAlive time.Time
	// Node's HAProxy host and port
	HAProxyHost string
	HAProxyPort uint
	// Data about all the FaaS functions. The key is the function name
	FuncsData map[string]*FuncData
}

// TableRecalc is actually a list of instances of the EntryRecalc struct, which can be
// accessed with concurrency-safe methods only
type TableRecalc struct {
	mutex   *sync.Mutex
	EntryValidity time.Duration
	// The list of the entries. The key is the node ID
	entries map[string]*EntryRecalc
}

//////////////////// PRIVATE METHODS ////////////////////

// InitTable initializes a TableRecalc's fields if they are empty
func (tbl *TableRecalc) initTable() {
	logger := logging.Logger()

	if tbl.entries == nil {
		tbl.entries = map[string]*EntryRecalc{}
		logger.Debug("Initialized table entries")
	}

	if tbl.mutex == nil {
		tbl.mutex = &sync.Mutex{}
		logger.Debug("Initialized table mutex")
	}
}

// isExpired returns true if the EntryRecalc is expired, according to
// tbl.EntryValidity and entry.TAlive
func (tbl *TableRecalc) isExpired(entry *EntryRecalc) bool {
	return entry.TAlive.Add(tbl.EntryValidity).Before(time.Now())
}

//////////////////// PUBLIC METHODS ////////////////////

// NewTableRecalc creates and initializes a new TableRecalc returning a pointer to the table
func NewTableRecalc(entryValidity time.Duration) *TableRecalc {
	tbl := &TableRecalc{
		EntryValidity: entryValidity,
	}
	tbl.initTable()

	return tbl
}

// SafeExec lets you execute a generic operation on the TableRecalc entries atomically
// (in a critical section). You can safely pass the entries map by value to
// function, instead of a pointer, because a map is itself a pointer type
func (tbl *TableRecalc) SafeExec(function func(entries map[string]*EntryRecalc) error) error {
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
