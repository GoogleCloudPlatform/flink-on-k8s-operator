/*
Copyright 2019 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flinkclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

// HTTPClient - HTTP client.
type HTTPClient struct {
	Log logr.Logger
}

// Get - HTTP GET.
func (c *HTTPClient) Get(url string, outStructPtr interface{}) error {
	return c.doHTTP("GET", url, nil, outStructPtr)
}

// Post - HTTP POST.
func (c *HTTPClient) Post(
	url string, body []byte, outStructPtr interface{}) error {
	return c.doHTTP("POST", url, body, outStructPtr)
}

// Patch - HTTP PATCH.
func (c *HTTPClient) Patch(
	url string, body []byte, outStructPtr interface{}) error {
	return c.doHTTP("PATCH", url, body, outStructPtr)
}

func (c *HTTPClient) doHTTP(
	method string, url string, body []byte, outStructPtr interface{}) error {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := c.createRequest(method, url, body)
	c.Log.Info("HTTPClient", "url", url, "method", method, "error", err)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	c.Log.Info(
		"HTTPClient", "status", resp.Status, "body", outStructPtr, "error", err)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("%v", resp.Status)
		return err
	}
	return c.readResponse(resp, outStructPtr)
}

func (c *HTTPClient) createRequest(
	method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flink-operator")
	return req, err
}

func (c *HTTPClient) readResponse(
	resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		err = json.Unmarshal(body, out)
	}
	return err
}
