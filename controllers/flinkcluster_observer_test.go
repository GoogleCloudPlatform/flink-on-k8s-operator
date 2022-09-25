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

package controllers

import (
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestGetFlinkJobDeploymentState(t *testing.T) {
	var pod corev1.Pod
	var submitterLog, expected *SubmitterLog
	var termMsg string
	var observer = ClusterStateObserver{}

	// success
	termMsg = `
jobID: ec74209eb4e3db8ae72db00bd7a830aa
message: |
  Successfully submitted!
  /opt/flink/bin/flink run --jobmanager flinkjobcluster-sample-jobmanager:8081 --class org.apache.flink.streaming.examples.wordcount.WordCount --parallelism 2 --detached ./examples/streaming/WordCount.jar --input ./README.txt
  Starting execution of program
  Printing result to stdout. Use --output to specify output path.
  Job has been submitted with JobID ec74209eb4e3db8ae72db00bd7a830aa
`
	expected = &SubmitterLog{
		JobID: "ec74209eb4e3db8ae72db00bd7a830aa",
		Message: `Successfully submitted!
/opt/flink/bin/flink run --jobmanager flinkjobcluster-sample-jobmanager:8081 --class org.apache.flink.streaming.examples.wordcount.WordCount --parallelism 2 --detached ./examples/streaming/WordCount.jar --input ./README.txt
Starting execution of program
Printing result to stdout. Use --output to specify output path.
Job has been submitted with JobID ec74209eb4e3db8ae72db00bd7a830aa
`,
	}
	pod = corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: termMsg,
					}}}}}}
	submitterLog = new(SubmitterLog)
	_ = observer.observeFlinkJobSubmitterLog(&pod, submitterLog)
	assert.DeepEqual(t, *submitterLog, *expected)
}
