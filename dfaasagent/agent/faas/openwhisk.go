// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or-later license. See LICENSE and
// AUTHORS file for more information.

package faas

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// owAnnotation represents a single OpenWhisk action annotation.
//
// Value is json.RawMessage because annotation values can have different JSON
// types, including strings, numbers, booleans, and objects.
type owAnnotation struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// owAction contains the relevant attributes of an action returned by the
// OpenWhisk actions endpoint.
type owAction struct {
	// Namespace is "<namespace>/<package>" for actions inside a package,
	// otherwise is only "<namespace>".
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Annotations []owAnnotation `json:"annotations"`
}

// qualifiedName returns the action name in the "<package>/<action>" form, as
// used in Web Action URLs. Actions not in a package belong to the "default"
// package.
func (a owAction) qualifiedName() string {
	pkg := "default"
	if _, p, found := strings.Cut(a.Namespace, "/"); found {
		pkg = p
	}
	return pkg + "/" + a.Name
}

type owActionsResponse []owAction

// annotation returns the string representation of the annotation value.
// JSON strings are unquoted; other JSON values are returned as raw JSON.
// It returns "", false if the annotation is not found.
func (a owAction) annotation(key string) (string, bool) {
	for _, ann := range a.Annotations {
		if ann.Key != key {
			continue
		}

		var value string
		if err := json.Unmarshal(ann.Value, &value); err == nil {
			return value, true
		}

		return string(ann.Value), true
	}

	return "", false
}

// OpenWhisk holds information for connecting to the local OpenWhisk instance.
type OpenWhisk struct {
	httpClient *http.Client
	address    string
	namespace  string
	apiKey     string
}

// NewOpenWhisk returns a new OpenWhisk object.
func NewOpenWhisk(hostname string, port uint, namespace, apiKey string) (*OpenWhisk, error) {
	namespace = strings.TrimSpace(namespace)
	apiKey = strings.TrimSpace(apiKey)

	if namespace == "" {
		return nil, fmt.Errorf("expected a valid OpenWhisk namespace, got empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("expected a valid OpenWhisk API key, got empty")
	}

	return &OpenWhisk{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		address:    net.JoinHostPort(hostname, strconv.FormatUint(uint64(port), 10)),
		namespace:  namespace,
		apiKey:     apiKey,
	}, nil
}

// actions retrieves action information from the OpenWhisk actions endpoint.
//
// Only Web Actions are supported!
func (o *OpenWhisk) actions() (owActionsResponse, error) {
	u := url.URL{
		Scheme: "http",
		Host:   o.address,
		Path:   "/api/v1/namespaces/" + url.PathEscape(o.namespace) + "/actions",
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building OpenWhisk actions request: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(o.apiKey))
	req.Header.Set("Authorization", "Basic "+encoded)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET to OpenWhisk actions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP GET to OpenWhisk actions: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP response from OpenWhisk actions: %w", err)
	}

	var actions owActionsResponse
	if err := json.Unmarshal(body, &actions); err != nil {
		return nil, fmt.Errorf("deserializing JSON actions info: %w", err)
	}

	// Keep only Web Actions. Since action names are mapped to HAProxy
	// identifiers, we also make sure that two actions do not end up with the
	// same identifier.
	webActions := make(owActionsResponse, 0, len(actions))
	proxyIDs := make(map[string]string, len(actions))
	for _, action := range actions {
		if value, ok := action.annotation("web-export"); !ok || value != "true" {
			continue
		}

		name := action.qualifiedName()
		id := ProxyID(name)
		if other, found := proxyIDs[id]; found {
			return nil, fmt.Errorf("actions %q and %q have the same proxy ID %q", other, name, id)
		}
		proxyIDs[id] = name

		webActions = append(webActions, action)
	}

	return webActions, nil
}

func (o *OpenWhisk) Platform() string {
	return PlatformOpenWhisk
}

// PathPrefix returns the Web Actions path prefix for the configured namespace.
// The namespace must be explicit (not "_") to build a valid URL.
func (o *OpenWhisk) PathPrefix() string {
	return "/api/v1/web/" + o.namespace + "/"
}

// Path returns the Web Action path for the given action, expected in the
// "<package>/<action>" form.
func (o *OpenWhisk) Path(name string) string {
	return o.PathPrefix() + name
}

// Names returns the Web Action names in the "<package>/<action>" form.
func (o *OpenWhisk) Names() ([]string, error) {
	actions, err := o.actions()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.qualifiedName())
	}

	return names, nil
}

func (o *OpenWhisk) WithMaxRates() (map[string]uint, error) {
	actions, err := o.actions()
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint, len(actions))
	for _, action := range actions {
		value, ok := action.annotation("dfaas.maxrate")
		if !ok {
			continue
		}

		maxRate, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing MaxRate for action %q: %w", action.qualifiedName(), err)
		}

		result[action.qualifiedName()] = uint(maxRate)
	}

	return result, nil
}

func (o *OpenWhisk) WithTimeout() (map[string]*uint, error) {
	actions, err := o.actions()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*uint, len(actions))
	for _, action := range actions {
		value, ok := action.annotation("dfaas.timeout_ms")
		if !ok {
			result[action.qualifiedName()] = nil
			continue
		}

		timeout, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing Timeout for action %q: %w", action.qualifiedName(), err)
		}

		v := uint(timeout)
		result[action.qualifiedName()] = &v
	}

	return result, nil
}

func (o *OpenWhisk) HealthCheck() (string, error) {
	u := url.URL{
		Scheme: "http",
		Host:   o.address,
		Path:   "/api/v1/namespaces",
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("building OpenWhisk health check request: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(o.apiKey))
	req.Header.Set("Authorization", "Basic "+encoded)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenWhisk health check: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	return resp.Status, nil
}
