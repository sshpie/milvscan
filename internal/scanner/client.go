package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpClient struct {
	baseURL string
	token   string
	http    *http.Client
	verbose bool
}

func newClient(baseURL, token string, timeoutSec int, verbose bool) *httpClient {
	return &httpClient{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
			// DisableKeepAlives ensures each request gets a fresh TCP connection.
			// Without this, a timed-out request leaves a stale half-open connection
			// in the pool, causing all subsequent requests to the same host to also
			// time out (connection reuse hits the dead socket).
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		verbose: verbose,
	}
}

// setHeaders applies Bearer token auth and content-type when needed.
// Milvus reads auth from "Authorization: Bearer <token>".
func (c *httpClient) setHeaders(req *http.Request, hasBody bool) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
}

// get fetches a path and decodes JSON into out. Returns (statusCode, error).
func (c *httpClient) get(path string, out interface{}) (int, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	c.setHeaders(req, false)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if c.verbose {
		fmt.Printf("  GET %s -> %d (%d bytes)\n", path, resp.StatusCode, len(body))
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, json.Unmarshal(body, out)
}

// getRaw fetches a path and returns the status code and raw body.
func (c *httpClient) getRaw(path string) (int, []byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	c.setHeaders(req, false)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if c.verbose {
		fmt.Printf("  GET %s -> %d (%d bytes)\n", path, resp.StatusCode, len(body))
	}
	return resp.StatusCode, body, nil
}

// post sends JSON body to path and decodes the response into out.
func (c *httpClient) post(path string, payload interface{}, out interface{}) (int, error) {
	return c.send("POST", path, payload, out)
}

func (c *httpClient) send(method, path string, payload interface{}, out interface{}) (int, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	c.setHeaders(req, true)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if c.verbose {
		fmt.Printf("  %s %s -> %d (%d bytes)\n", method, path, resp.StatusCode, len(body))
	}
	if out != nil {
		return resp.StatusCode, json.Unmarshal(body, out)
	}
	return resp.StatusCode, nil
}
