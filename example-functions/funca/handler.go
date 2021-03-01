package function

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	handler "github.com/openfaas/templates-sdk/go-http"
)

// The value of nodeNum is automatically replaced during deployment
const nodeNum = 0 // ###NODENUM###

// The value of nodeName is automatically replaced during deployment
const nodeName = "###NODENAME###"

// The value of funcName is automatically replaced during deployment
const funcName = "###FUNCNAME###"

var ignoredReqHeaders = [...]string{
	"Accept",
	"Accept-Encoding",
	"Accept-Language",
	"Cache-Control",
	"Pragma",
	"Upgrade-Insecure-Requests",
	"User-Agent",
}

// getIgnoreHeader returns true if the header must not be printed out
func getIgnoreHeader(givenHdr string) bool {
	for _, x := range ignoredReqHeaders {
		if strings.ToUpper(x) == strings.ToUpper(givenHdr) {
			return true
		}
	}
	return false
}

// Handle a function invocation
func Handle(req handler.Request) (handler.Response, error) {
	var b strings.Builder

	//////////////////////////////////////////////////

	b.WriteString("Hello from function ")
	b.WriteString(funcName)
	b.WriteString(" @ OpenFaaS gateway ")
	b.WriteString(nodeName)
	b.WriteString("!\n\n")

	//////////////////////////////////////////////////

	b.WriteString("Timestamp: ")
	b.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	b.WriteString("\n")

	b.WriteString("Host: ")
	b.WriteString(req.Host)
	b.WriteString("\n")

	b.WriteString("QueryString: ")
	b.WriteString(req.QueryString)
	b.WriteString("\n\n")

	//////////////////////////////////////////////////

	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("Relevant request headers:\n")
	for _, key := range keys {
		if getIgnoreHeader(key) {
			continue
		}
		for _, val := range req.Header[key] {
			b.WriteString("  - ")
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(val)
			b.WriteString("\n")
		}
	}

	//////////////////////////////////////////////////

	return handler.Response{
		Body:       []byte(b.String()),
		StatusCode: http.StatusOK + 60 + nodeNum, // Different response status code for each node! e.g. 261, 262, 263
	}, nil
}
