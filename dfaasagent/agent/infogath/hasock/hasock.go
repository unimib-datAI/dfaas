// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package hasock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bcicen/go-haproxy"
	"github.com/pkg/errors"
)

// STEntry represents a row of a stick-table
type STEntry struct {
	HTTPReqCnt  uint
	HTTPReqRate uint
}

type haproxyAPIEntry struct {
	Key         string `json:"key"`
	HTTPReqCnt  uint   `json:"http_req_cnt"`
	HTTPReqRate uint   `json:"http_req_rate"`
}

var _reStickTable *regexp.Regexp = nil

// Note: to view the real-time content of a stick table directly from bash, use
// the following command:
// watch "echo show table st_src_global | socat stdio dfaasvolume1/haproxy.sock"

// ReadStickTable reads the content of a stick table from an HAProxy socket
// client. The stick table must be of type "http_req_cnt,http_req_rate(1s)"
func ReadStickTable(hasockClient *haproxy.HAProxyClient, stName string) (map[string]*STEntry, error) {

	//Retreive isKube from configmap
	val := os.Getenv("IS_KUBE")
	isKube, err := strconv.ParseBool(val)
	if err != nil {
		fmt.Printf("Invalid IS_KUBE value: %s\n", val)
		isKube = false
	}

	if isKube {
		baseURL := "http://haproxy-service:5555/v3/services/haproxy/runtime"
		client := &http.Client{}

		//Get list of stick tables
		listURL := fmt.Sprintf("%s/stick_tables", baseURL)
		req, err := http.NewRequest("GET", listURL, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create request for stick table list")
		}
		req.SetBasicAuth("admin", "mypassword")

		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to call HAProxy stick_table list API")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HAProxy list API returned %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var tables []struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tables); err != nil {
			return nil, errors.Wrap(err, "failed to decode stick_table list response")
		}

		//Check if stName is in the list
		found := false
		for _, t := range tables {
			if t.Name == stName {
				found = true
				break
			}
		}
		if !found {
			return map[string]*STEntry{}, nil
		}

		// Fetch stick table entries
		entriesURL := fmt.Sprintf("%s/stick_tables/%s/entries", baseURL, stName)
		req, err = http.NewRequest("GET", entriesURL, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create request for stick table entries")
		}
		req.SetBasicAuth("admin", "mypassword")

		resp, err = client.Do(req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to call HAProxy stick_table entries API")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HAProxy entries API returned %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var entries []haproxyAPIEntry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			return nil, errors.Wrap(err, "failed to parse entries JSON")
		}

		// Convert to result format
		result := make(map[string]*STEntry)
		for _, entry := range entries {
			result[entry.Key] = &STEntry{
				HTTPReqCnt:  entry.HTTPReqCnt,
				HTTPReqRate: entry.HTTPReqRate,
			}
		}

		return result, nil
	} else {

		cmd := fmt.Sprintf("show table %s", stName)

		buf, err := hasockClient.RunCommand(cmd)
		if err != nil {
			return nil, errors.Wrap(err, "Error while requesting content of a stick-table from the HAProxy socket")
		}

		if _reStickTable == nil {
			_reStickTable, err = regexp.Compile("^0x[0-9a-f]+: key=([^ ]+) use=[0-9]+ exp=[0-9]+ http_req_cnt=([0-9]+) http_req_rate\\(1000\\)=([0-9]+)$")
			if err != nil {
				return nil, errors.Wrap(err, "Error while compiling the regular expression for the HAProxy socket response parsing")
			}
		}
		response := strings.Split(buf.String(), "\n")
		result := map[string]*STEntry{}

		for _, line := range response {
			matches := _reStickTable.FindStringSubmatch(line)
			if matches != nil && len(matches) > 0 {
				key := matches[1]

				cnt, err := strconv.ParseUint(matches[2], 10, 32)
				if err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("Error while parsing integer from the HAProxy socket response: %d", cnt))
				}

				rate, err := strconv.ParseUint(matches[3], 10, 32)
				if err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("Error while parsing integer from the HAProxy socket response: %d", rate))
				}

				result[key] = &STEntry{
					HTTPReqCnt:  uint(cnt),
					HTTPReqRate: uint(rate),
				}
			}
		}
		return result, nil
	}

}
