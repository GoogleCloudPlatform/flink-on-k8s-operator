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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

// JobStatus defines Flink job status.
type JobStatus struct {
	ID     string
	Status string
}

// JobStatusList defines Flink job status list.
type JobStatusList struct {
	Jobs []JobStatus
}

// GetJobStatusList gets Flink job status list.
func GetJobStatusList(url string, jobStatusList *JobStatusList) error {
	var err = httpGet(url, jobStatusList)
	return err
}

func httpGet(url string, outStructPtr interface{}) error {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flink-operator")
	resp, err := httpClient.Do(req)
	if err == nil {
		err = readResponse(resp, outStructPtr)
	}
	return err
}

func readResponse(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		err = json.Unmarshal(body, out)
	}
	return err
}
