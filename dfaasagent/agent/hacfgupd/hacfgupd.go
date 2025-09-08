// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// Package hacfgup updates the HAProxy configuration.
//
// The DFaaS agent communicates with HAProxy through the Data Plane API. That
// service will then restart HAProxy with the new configuration.
package hacfgupd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"text/template"

	"github.com/Masterminds/sprig"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

// Updater is the main type for updating an HAProxy configuration using a
// template.
type Updater struct {
	// Loaded template to use for writing the HAProxy config file.
	template *template.Template
}

// LoadTemplate loads the template from the given string.
func (updater *Updater) LoadTemplate(templateContent string) error {
	// Create a new empty template without name.
	tmpl := template.New("")

	// Add sprig functions to the template.
	tmpl = tmpl.Funcs(sprig.TxtFuncMap())

	// Parse template content.
	tmpl, err := tmpl.Parse(templateContent)
	if err != nil {
		return fmt.Errorf("loading HAProxy configuration template from content: %w", err)
	}

	updater.template = tmpl

	return nil
}

// UpdateHAConfig updates the HAProxy config file and posts it using an
// in-memory buffer.
func (updater *Updater) UpdateHAConfig(content interface{}) error {
	logger := logging.Logger()

	// Apply the template to a string buffer in RAM.
	var buf bytes.Buffer
	err := updater.template.Execute(&buf, content)
	if err != nil {
		return fmt.Errorf("applying the HAProxy configuration template to the data: %w", err)
	}
	configData := buf.Bytes()

	// Create POST request.
	url := fmt.Sprintf("%s/v3/services/haproxy/configuration/raw?skip_version=true", constants.HAProxyDataPlaneAPIOrigin)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(configData))
	if err != nil {
		return fmt.Errorf("creating HTTP request to Data Plane API: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth(constants.HAProxyDataPlaneUsername, constants.HAProxyDataPlanePassword)

	// Send POST request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to Data Plane API: %w", err)
	}
	defer resp.Body.Close()

	// Get response.
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading Data Plane API response: %w", err)
	}

	logger.Debug(fmt.Sprintf("Data Plane API response %q: %s", resp.Status, string(rawBody)))

	return nil
}
