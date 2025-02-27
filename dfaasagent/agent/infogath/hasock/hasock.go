// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package hasock

import (
	"fmt"
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

var _reStickTable *regexp.Regexp = nil

// Note: to view the real-time content of a stick table directly from bash, use
// the following command:
// watch "echo show table st_src_global | socat stdio dfaasvolume1/haproxy.sock"

// ReadStickTable reads the content of a stick table from an HAProxy socket
// client. The stick table must be of type "http_req_cnt,http_req_rate(1s)"
func ReadStickTable(hasockClient *haproxy.HAProxyClient, stName string) (map[string]*STEntry, error) {
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
