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
	"os"
	"path"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

// Connection information for accessing the Data Plane API.
const (
	Origin   = "http://haproxy.default.svc.cluster.local:5555"
	Username = "admin"
	Password = "admin"
)

// Updater is the main type for updating an HAProxy configuration file using a
// template.
type Updater struct {
	// Loaded template to use for writing the HAProxy config file.
	template *template.Template

	// Path to the HAProxy config file to write.
	HAConfigFilePath string
}

// LoadTemplate Loads the template from file
func (updater *Updater) LoadTemplate(templateFilePath string) error {
	tmpl := template.New(path.Base(templateFilePath)) // Create new empty template
	tmpl = tmpl.Funcs(sprig.TxtFuncMap())             // Add sprig functions
	tmpl, err := tmpl.ParseFiles(templateFilePath)    // Parse the template file
	if err != nil {
		return errors.Wrap(err, "Error while loading HAProxy configuration template from file")
	}

	updater.template = tmpl

	return nil
}

// UpdateHAConfig updates the HAProxy config file
func (updater *Updater) UpdateHAConfig(content interface{}) error {
	logger := logging.Logger()

	f, err := os.Create(updater.HAConfigFilePath)
	if err != nil {
		return errors.Wrap(err, "Error while opening the HAProxy configuration file for writing")
	}
	defer f.Close()

	err = updater.template.Execute(f, content)
	if err != nil {
		return errors.Wrap(err, "Error while applying the HAProxy configuration template to the data")
	}

	configData, err := os.ReadFile(updater.HAConfigFilePath)
	if err != nil {
		return errors.Wrap(err, "Error reading the generated HAProxy configuration file")
	}

	// Create POST request.
	url := fmt.Sprintf("%s/v3/services/haproxy/configuration/raw?skip_version=true", Origin)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(configData))
	if err != nil {
		return errors.Wrap(err, "Failed to create HTTP request to Data Plane API")
	}
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth(Username, Password)

	// Send POST request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send HTTP request to Data Plane API")
	}
	defer resp.Body.Close()

	// Get response.
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "Failed to read Data Plane API response")
	}

	logger.Debug("Data Plane API response",
		zap.String("status", resp.Status),
		zap.String("body", string(rawBody)))

	return nil
}
