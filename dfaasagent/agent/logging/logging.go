// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This packages handles logging in dfaasagent.
package logging

import (
	"errors"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger
var logfile *os.File
var flushStop chan struct{}
var flushDone chan struct{}

// Initialize configures the shared logger.
//
// Only logs at the configured level or above are written. The log level must be
// a valid string supported by zap, see: https://pkg.go.dev/go.uber.org/zap/zapcore#ParseLevel
//
// Logs are written to stdout in console format and to the specified file in
// JSON Lines format. An existing log file is truncated.
func Initialize(level string, filename string) error {
	if logger != nil {
		return fmt.Errorf("logger already initialized")
	}

	if filename == "" {
		return fmt.Errorf("expected log filename, got empty string")
	}

	logLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("parse log level %q: %w", level, err)
	}

	// Configure human-readable console output.
	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02T15:04:05.000"))
	}

	// Configure structured JSON output.
	jsonConfig := zap.NewProductionEncoderConfig()
	jsonConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	jsonConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	// Create or truncate the log file.
	openedFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", filename, err)
	}

	// Configure the output cores.
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleConfig),
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(jsonConfig),
		zapcore.AddSync(openedFile),
		logLevel,
	)

	core := zapcore.NewTee(consoleCore, fileCore)

	newLogger := zap.New(core, zap.AddCaller()).Sugar()

	// Save the initialized logger and file.
	logfile = openedFile
	logger = newLogger

	// Start periodic file synchronization.
	flushStop = make(chan struct{})
	flushDone = make(chan struct{})

	go func() {
		defer close(flushDone)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = logfile.Sync()
			case <-flushStop:
				return
			}
		}
	}()

	return nil
}

// Logger returns the shared logger.
// It returns nil if Initialize has not been called.
func Logger() *zap.SugaredLogger {
	return logger
}

// Sync flushes pending log entries and closes the log file.
// Call Sync after all goroutines have stopped logging.
func Sync() error {
	if logger == nil {
		return fmt.Errorf("logger not initialized")
	}

	// Stop the periodic flush goroutine and wait for it to exit.
	close(flushStop)
	<-flushDone

	var errs []error

	// Sync and close only the file, since stdout could not be synchronized.
	if logfile != nil {
		if err := logfile.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("sync log file: %w", err))
		}

		if err := logfile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close log file: %w", err))
		}
	}

	logger = nil
	logfile = nil

	return errors.Join(errs...)
}
