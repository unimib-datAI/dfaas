// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package maddrhelp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMAddrList(t *testing.T) {
	ass := require.New(t)

	var tests = []struct {
		list, sep string
		want      []string
		success   bool
	}{
		{" /ip4/1.2.3.4/tcp/1234", ",", []string{"/ip4/1.2.3.4/tcp/1234"}, true},
		{" \n /ip4/1.2.3.4/tcp/1234  \r  \n  , \t /ip4/5.6.7.8/tcp/5678,, ,", ",", []string{"/ip4/1.2.3.4/tcp/1234", "/ip4/5.6.7.8/tcp/5678"}, true},
		{" \n /ip4/1.2.3.4/tcp/1234  \n  \t\t /ip4/5.6.7.8/tcp/5678    \n", "\n", []string{"/ip4/1.2.3.4/tcp/1234", "/ip4/5.6.7.8/tcp/5678"}, true},
		{" \n /ip4/1.2.3.4/tcp/1234  \r\n  \t\t /ip4/5.6.7.8/tcp/5678\r\n", "\n", []string{"/ip4/1.2.3.4/tcp/1234", "/ip4/5.6.7.8/tcp/5678"}, true},
		{"/ejvivb/hvirebfhjk", ",", nil, false},
	}

	for _, tt := range tests {
		output, err := ParseMAddrList(tt.list, tt.sep)

		if tt.success {
			ass.Equal(nil, err)
		} else {
			ass.NotEqual(nil, err)
		}

		ass.Equal(len(tt.want), len(output))

		for i := range tt.want {
			ass.Equal(tt.want[i], output[i].String())
		}
	}
}
