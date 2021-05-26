package nodestbl

import (
	"sync"
	"time"
)

// This package is for handling the information about the other nodes in the
// network, such as the max rate limits and the address and port of the relative
// HAProxy server instance

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

// Entry represents a row in the table. It is relative to a node of the p2p
// network
type Entry struct {
	ID string

	HAProxyHost string
	HAProxyPort uint

	// Data about all the FaaS functions. The key is the function name
	FuncsData map[string]*FuncData

	// Time of the last message received from the node
	TAlive time.Time
}

// Table is actually a list of instances of the Entry struct, which can be
// accessed with concurrency-safe methods only
type Table struct {
	// The list of the entries. The key is the node ID
	entries map[string]*Entry
	mutex   *sync.Mutex

	EntryValidity time.Duration
}

//////////////////// PRIVATE METHODS ////////////////////

// isExpired returns true if the entry is expired, according to
// tbl.EntryValidity and entry.TAlive
func (tbl *Table) isExpired(entry *Entry) bool {
	return entry.TAlive.Add(tbl.EntryValidity).Before(time.Now())
}

//////////////////// PUBLIC METHODS ////////////////////

// InitTable initializes a Table's fields if they are empty
func (tbl *Table) InitTable() {
	if tbl.entries == nil {
		tbl.entries = map[string]*Entry{}
	}
	if tbl.mutex == nil {
		tbl.mutex = &sync.Mutex{}
	}
}

// SafeExec lets you execute a generic operation on the table entries atomically
// (in a critical section). You can safely pass the entries map by value to
// function, instead of a pointer, because a map is itself a pointer type
func (tbl *Table) SafeExec(function func(entries map[string]*Entry) error) error {
	tbl.mutex.Lock()
	defer tbl.mutex.Unlock()

	tbl.InitTable()

	// Remove expired entries before doing anything
	for nodeID, entry := range tbl.entries {
		if tbl.isExpired(entry) {
			delete(tbl.entries, nodeID)
		}
	}

	return function(tbl.entries)
}

// SetReceivedValues should be executed when the data from the node is received
// and must be stored in the table. Updates everything except LimitIn and
// NodeWeight for functions which were already present
func (tbl *Table) SetReceivedValues(
	nodeID string,
	haProxyHost string,
	haProxyPort uint,
	funcLimits map[string]float64,
) error {
	return tbl.SafeExec(func(entries map[string]*Entry) error {

		// If the message arrives from a sender node with ID nodeID that is
		// not present in _nodesbl yet, it is added to the table.
		_, present := entries[nodeID]
		if !present {
			entries[nodeID] = &Entry{
				FuncsData: map[string]*FuncData{},
			}
		}

		entries[nodeID].TAlive = time.Now()

		entries[nodeID].HAProxyHost = haProxyHost
		entries[nodeID].HAProxyPort = haProxyPort

		// Remove from my table the functions limits which are no more present
		// in the new updated message
		for funcName := range entries[nodeID].FuncsData {
			// Once this routine executed a message from another node of the
			// p2p net has been received.
			// If I (and I am the receiver node) stored in _nodestbl functions
			// that are not more present in sender node, identified by the fact
			// that they are not more present in received message, I can remove them
			// from my local table.
			_, present := funcLimits[funcName]
			if !present {
				delete(entries[nodeID].FuncsData, funcName)
			}
		}

		// Update the functions limits with the received values (also add new
		// functions which weren't present before)
		for funcName, limit := range funcLimits {
			_, present := entries[nodeID].FuncsData[funcName]
			if present {
				// For each function received by sender node, updates
				// corrisponding line of _nodestbl table.
				// If entry for sender node is present, updates LimitOut
				// for that node with received limit.
				// LimitOut means number of req/sec that I can fwd
				// toward this node.
				// Note: this LimitOut is updated on the base of LimitIn
				// for this function received by i-th node (sender).
				entries[nodeID].FuncsData[funcName].LimitOut = limit
			} else {
				entries[nodeID].FuncsData[funcName] = &FuncData{
					LimitIn:    0,
					LimitOut:   limit,
					NodeWeight: 0,
				}
			}
		}

		return nil
	})
}
