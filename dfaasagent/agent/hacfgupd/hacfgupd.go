// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package handles the HAProxy ConFiGuration UPDate process (caps are the
// meaning of the acronym)
package hacfgupd

import (
	"os"
	"os/exec"
	"path"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
)

// updateStdoutDebugLogging decides wheather to enable or disable stdout logging
// for the CmdOnUpdated command
const updateStdoutDebugLogging = false

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
	f, err := os.Create(updater.HAConfigFilePath)
	if err != nil {
		return errors.Wrap(err, "Error while opening the HAProxy configuration file for writing")
	}
	defer f.Close()

	err = updater.template.Execute(f, content)
	if err != nil {
		return errors.Wrap(err, "Error while applying the HAProxy configuration template to the data")
	}

	cmd := exec.Command("bash", "-c", updater.CmdOnUpdated)
	if updateStdoutDebugLogging {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "Error while executing the HAProxy configuration update command (command: \""+updater.CmdOnUpdated+"\")")
	}

	return nil
}
