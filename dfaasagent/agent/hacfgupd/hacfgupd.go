// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles the HAProxy ConFiGuration UPDate process (caps are the
// meaning of the acronym)
package hacfgupd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
)

// updateStdoutDebugLogging decides wheather to enable or disable stdout logging
// for the CmdOnUpdated command
const updateStdoutDebugLogging = true

// Updater is the main type for updating an HAProxy configuration file using a
// template
type Updater struct {
	template         *template.Template // Loaded template to use for writing the HAProxy config file
	HAConfigFilePath string             // Path to the HAProxy config file to write
	CmdOnUpdated     string             // Command to be executed after the HAProxy config file has been successfully written
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

	//Retreive isKube from configmap
	val := os.Getenv("IS_KUBE")
	isKube, err := strconv.ParseBool(val)
	if err != nil {
		fmt.Printf("Invalid IS_KUBE value: %s\n", val)
		isKube = false
	}

	if isKube {
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

		req, err := http.NewRequest("POST", "http://haproxy-service:5555/v3/services/haproxy/configuration/raw?skip_version=true", bytes.NewBuffer(configData))
		if err != nil {
			return errors.Wrap(err, "Failed to create HTTP request to Data Plane API")
		}
		req.Header.Set("Content-Type", "text/plain")

		username := "admin"
		password := "mypassword"
		auth := username + ":" + password
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrap(err, "Failed to send HTTP request to Data Plane API")
		}
		defer resp.Body.Close()

		// Read and print response
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "Error while executing the HAProxy configuration update command")
		}

		fmt.Println("Response status:", resp.Status)
		fmt.Println("Response body:", string(respBody))
	} else {
		cmd := exec.Command("bash", "-c", updater.CmdOnUpdated)
		if updateStdoutDebugLogging {
			cmd.Stdout = os.Stdout
		}
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "Error while executing the HAProxy configuration update command (command: \""+updater.CmdOnUpdated+"\")")
		}
	}

	return nil
}
