// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"encoding/json"
	"os"
	"io/ioutil"

	"github.com/pkg/errors"
)

// The Node Margin Strategy expects to have as input models the number of
// requests that are predicted to occur for each group of functions. This
// allows the models not to be trained for each specific function.
//
// The problem is how to assign each function to a group. For now, this is not
// a problem for the DFaaS agent, we expect that there is an externally
// provided file that contains the list of functions for each class.
//
// This file defines a struct type and a function to read the file as get the
// function groups.

//////////////////// PUBLIC STRUCT TYPES ////////////////////

// Represents the possible function groups with a list of functions for each
// group.
type Groups struct {
	// High usage functions.
	HighUsage 	[]string `json:"HIGH_USAGE"`

	// Medium usage functions.
	MediumUsage []string `json:"MEDIUM_USAGE"`

	// Low usage functions.
	LowUsage 	[]string `json:"LOW_USAGE"`
}

//////////////////// PUBLIC METHODS ////////////////////

// GetFuncsGroups returns the function groups.
//
// The groups are read from a JSON file specified as GroupListFileName in the
// configuration.
func GetFuncsGroups() (Groups, error) {
	groupListFile := _config.GroupListFileName

	// TODO: use os.ReadFile instead of os.Open, ioutil.ReadAll and io.Close.
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
