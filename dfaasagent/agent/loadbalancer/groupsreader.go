package loadbalancer

import (
	"encoding/json"
	"os"
	"io/ioutil"

	"github.com/pkg/errors"
)

// In this file are implemented functionalities to read function groups from the group_list.json file,
// where, for each group (high usage, medium usage, low usage), there is a list
// of functions belonging to that group

//////////////////// PUBLIC STRUCT TYPES ////////////////////

// Struct to read functions groups from group_list.json
type Groups struct {
	// Array of high usage functions
	HighUsage 	[]string `json:"HIGH_USAGE"`
	// Array of medium usage functions
	MediumUsage []string `json:"MEDIUM_USAGE"`
	// Array of low usage functions
	LowUsage 	[]string `json:"LOW_USAGE"`
}

//////////////////// PUBLIC METHODS ////////////////////

// GetFuncsGroups reads from the group_list.json file specified in the configuration
// the functions groups, returning them in a variable of type Groups.
func GetFuncsGroups() (Groups, error) {
	groupListFile := _config.GroupListFileName
	jsonGroupsFile, err := os.Open(groupListFile)
	if err != nil {
		return Groups{}, errors.Wrap(err, "Error while reading group list json file")
	}
	jsonGroups, err := ioutil.ReadAll(jsonGroupsFile)
	if err != nil {
		return Groups{}, errors.Wrap(err, "Error while converting group list json file into byte array")
	}
	defer jsonGroupsFile.Close()

	var functionGroups Groups
	err = json.Unmarshal(jsonGroups, &functionGroups)
	if err != nil {
		return Groups{}, errors.Wrap(err, "Error while converting json groups")
	}

	return functionGroups, nil
}
