// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package is for getting the functions list from the local OpenFaaS
// cluster.
package offuncs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
)

/*
	Example of response from "/system/functions":

	[
		{
			"name": "nodeinfo",
			"image": "functions/nodeinfo-http:latest",
			"invocationCount": 0,
			"replicas": 1,
			"envProcess": "",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "nodeinfo",
				"com.openfaas.uid": "181161398",
				"dfaas.maxrate": "50",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "figlet",
			"image": "functions/figlet:0.13.0",
			"invocationCount": 0,
			"replicas": 1,
			"envProcess": "figlet",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "figlet",
				"com.openfaas.uid": "433840237",
				"dfaas.maxrate": "50",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funca",
			"image": "funca:latest",
			"invocationCount": 7501,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funca",
				"com.openfaas.uid": "107584837",
				"dfaas.maxrate": "110",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funcc",
			"image": "funcc:latest",
			"invocationCount": 2,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funcc",
				"com.openfaas.uid": "379141780",
				"dfaas.maxrate": "310",
				"function": "true"
			},
			"annotations": null
		},
		{
			"name": "funcb",
			"image": "funcb:latest",
			"invocationCount": 1,
			"replicas": 1,
			"envProcess": "./handler",
			"availableReplicas": 0,
			"labels": {
				"com.openfaas.function": "funcb",
				"com.openfaas.uid": "764206021",
				"dfaas.maxrate": "210",
				"function": "true"
			},
			"annotations": null
		}
	]
*/

// funcsMaxRatesResponse is the structure of a response from "/system/functions". This
// contains only the relevant attributes (it is incomplete). It is used for "recalc" algorithm
type funcsMaxRatesResponse []struct {
	Name   string `json:"name"`
	Labels struct {
		MaxRate string `json:"dfaas.maxrate"`
	} `json:"labels"`
}

// funcsNamesResponse is the structure of a response from "/system/functions". This
// contains only the function names (it is incomplete)
type funcsNamesResponse []struct {
	Name string `json:"name"`
}

// Client for gathering information from OpenFaaS
type Client struct {
	Hostname string
	Port     uint
	Username string
	Password string
}

////////////////////////////////// PRIVATE FUNCTIONS ////////////////////////////////

// doFuncsRequest gets info about functions from "/system/functions". 
// The hostnameAndPort parameter can be like
// "myhostname:8080" or "myhostname" (implicit port 80) "192.168.15.101:8080"
// (specifying the IP address)
func (client *Client) doFuncsRequest() ([]byte, error) {
	strURL := fmt.Sprintf(
		"http://%s:%s@%s:%d/system/functions",
		client.Username,
		client.Password,
		client.Hostname,
		client.Port,
	)

	resp, err := http.Get(strURL)
	if err != nil {
		return nil, errors.Wrap(err, "Error while performing an HTTP GET request to /system/functions")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Error while reading the content of an HTTP response from /system/functions")
	}

	return body, nil
}

/////////////////////////////////// PUBLIC INTERFACE ////////////////////////////////

// GetFuncsWithMaxRates returns the functions list as a map[string]uint of function names
// and dfaas.maxrate values.
func (client *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
	body, err := client.doFuncsRequest()
	if err != nil {
		return nil, errors.Wrap(err, "Error while reading the content of an HTTP response from /system/functions")
	}

	var respObj funcsMaxRatesResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from /system/functions")
	}

	result := map[string]uint{}
	for _, item := range respObj {
		num, err := strconv.ParseUint(item.Labels.MaxRate, 10, 32)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("Error while parsing integer from /system/functions: %d", num))
		}
		result[item.Name] = uint(num)
	}

	return result, nil
}


// GetFuncsNames returns the function names list as a string array.
func (client *Client) GetFuncsNames() ([]string, error) {
	body, err := client.doFuncsRequest()
	if err != nil {
		return nil, errors.Wrap(err, "Error while reading the content of an HTTP response from /system/functions")
	}

	var respObj funcsNamesResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from /system/functions")
	}

	var result []string
	for _, item := range respObj {
		result = append(result, item.Name)
	}

	return result, nil
}
