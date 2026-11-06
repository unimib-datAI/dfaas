// Entry point of the OpenWhisk Go action. The runtime calls Main(args) with the
// Web Action parameters and expects a map with statusCode, headers and body as
// response. There is no func main because the runtime provides it.
//
// See: https://github.com/apache/openwhisk/blob/master/docs/actions-go.md
// See: https://github.com/apache/openwhisk/blob/master/docs/webactions.md
package main

func Main(args map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"statusCode": 200,
		"headers":    map[string]interface{}{"Content-Type": "text/plain; charset=utf-8"},
		"body":       "OK\n",
	}
}
