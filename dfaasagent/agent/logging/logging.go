// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This packages handles logging in dfaasagent
package logging

import (
	"time"

	"github.com/pkg/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _zapConfig zap.Config
var _zapLogger *zap.SugaredLogger

var _datetime bool
var _debugMode bool
var _colors bool

// Initialize builds and returns the logger for dfaasagent. With the datetime
// parameter, you can specify if you want the datetime to be prefixed at each
// log entry. With the debugMode parameter you can choose the logging level
// (INFO or DEBUG). With the colors parameter you can enable or disable colors
// for logging levels in output.
func Initialize(datetime bool, debugMode bool, colors bool) (*zap.SugaredLogger, error) {
	zapConfig := zap.NewProductionConfig()

	// Use human-readable messages instead of JSON
	zapConfig.Encoding = "console"

	// Disable stack trace output if not in debug mode
	zapConfig.DisableStacktrace = !debugMode

	if datetime {
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		// Empty time encoder function (to disable date/time logging)
		zapConfig.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {}
	}

	if debugMode {
		zapConfig.Level.SetLevel(zapcore.DebugLevel)
	} else {
		zapConfig.Level.SetLevel(zapcore.InfoLevel)
	}

	if colors {
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	unsugared, err := zapConfig.Build()
	if err != nil {
		return nil, errors.Wrap(err, "Error while constructing a logger")
	}

	// The logger will always be a sugared one
	zapLogger := unsugared.Sugar()

	// If everything successful, set the package's static vars
	_zapConfig = zapConfig
	_zapLogger = zapLogger
	_datetime = datetime
	_debugMode = debugMode
	_colors = colors

	return zapLogger, nil
}

// Logger returns the logger for dfaasagent. If it has not been initialized, it
// returns nil
func Logger() *zap.SugaredLogger {
	return _zapLogger
}

// GetDatetime returns true if the logger is configured to output date and time,
// false otherwise
func GetDatetime() bool {
	return _datetime
}

// GetDebugMode returns true if the logger level is set to DEBUG, false
// otherwise
func GetDebugMode() bool {
	return _debugMode
}

// GetColors returns true if the logger is configured to output the level names
// colored, false otherwise
func GetColors() bool {
	return _colors
}
