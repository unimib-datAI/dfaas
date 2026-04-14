package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Listen string
	Target string
	Map    string
}

func loadMapping(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	err = json.Unmarshal(b, &m)
	return m, err
}

func main() {
	var cfg Config

	flag.StringVar(&cfg.Listen, "listen", ":8080", "listen address")
	flag.StringVar(&cfg.Target, "target", "", "upstream target (e.g. http://localhost:9000)")
	flag.StringVar(&cfg.Map, "map", "map.json", "JSON mapping file")
	flag.Parse()

	if cfg.Target == "" {
		log.Fatal("missing -target")
	}

	mapping, err := loadMapping(cfg.Map)
	if err != nil {
		log.Fatalf("failed to load map: %v", err)
	}
	log.Printf("Loaded mapping: %v", mapping)

	targetURL, err := url.Parse(cfg.Target)
	if err != nil {
		log.Fatalf("invalid target URL: %v", err)
	}

	// We build a slice with [src1, dst1, src2, dst2...]. This is needed to
	// build a Replacer.
	pairs := make([]string, 0, len(mapping)*2)
	for src, dst := range mapping {
		pairs = append(pairs, src, dst)
	}
	replacer := strings.NewReplacer(pairs...)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		log.Printf("%s - %s %s", req.RemoteAddr, req.Method, req.URL.String())

		if req.Body == nil {
			return
		}

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Printf("read body error: %v", err)
			return
		}
		req.Body.Close()

		if len(bodyBytes) == 0 {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			return
		}

		newBody := replacer.Replace(string(bodyBytes))

		var buf bytes.Buffer
		buf.WriteString(newBody)

		req.Body = io.NopCloser(&buf)
		req.ContentLength = int64(buf.Len())
	}

	// Apply a similar tranformation also for the response, but with the mapping
	// reversed.
	reversePairs := make([]string, 0, len(mapping)*2)
	for src, dst := range mapping {
		reversePairs = append(reversePairs, dst, src)
	}
	reverseReplacer := strings.NewReplacer(reversePairs...)

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Body == nil {
			return nil
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()

		newBody := reverseReplacer.Replace(string(bodyBytes))

		var buf bytes.Buffer
		buf.WriteString(newBody)

		resp.Body = io.NopCloser(&buf)
		resp.ContentLength = int64(buf.Len())

		// Force Go to update the header with value from resp.ContentLength.
		resp.Header.Del("Content-Length")

		return nil
	}

	log.Printf("Proxy listening on %q with target %q", cfg.Listen, cfg.Target)
	log.Fatal(http.ListenAndServe(cfg.Listen, proxy))
}
