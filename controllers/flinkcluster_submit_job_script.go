/*
Copyright 2020 Google LLC.

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

// This script is part of the cluster's ConfigMap and is mounted into the
// job (submitter) container at `/opt/flink-operator/submit-job.sh` for job
// submission.
var submitJobScript = `
#! /usr/bin/env bash

# This script submits a job to the Flink cluster and waits until it finishes.
#
# It is a wrapper of the Flink CLI, but with additional checks to make sure in
# the event of failure recovery (e.g., job pod failure), it doesn't resubmit
# blindly but check if there is already one found in Flink. If found, it simply
# waits on the job.

set -euo pipefail

function check_job_not_submitted() {
  local -r MAX_RETRY=2
  for ((i = 1; i <= MAX_RETRY; i++)); do
    echo -e "curl -sS http://${FLINK_JM_ADDR}/jobs"
    if curl -sS "http://${FLINK_JM_ADDR}/jobs" >job_check_log; then
      if grep -e "\"id\":\"${FLINK_JOB_ID}\"" <job_check_log; then
        echo -e "\nFound the job. Job is already submitted with provided ID."
        return 1
      else
        cat job_check_log
        echo -e "\n\nNot found the job. The job is not submitted yet."
        return 0
      fi
    else
      echo -e "\nFailed to get the job status. The job status is unknown."
    fi
    if ((i < MAX_RETRY)); then
      echo -e "Will retry $((i))/$((MAX_RETRY - 1)) in 5 seconds."
      sleep 2
    fi
  done

  return 1
}

function upload_jar() {
  echo "curl -sS -X POST -H \"Expect:\" -F \"jarfile=@${FLINK_JOB_JAR_PATH}\" http://${FLINK_JM_ADDR}/jars/upload"
  if curl -sS -X POST -H "Expect:" -F "jarfile=@${FLINK_JOB_JAR_PATH}" "http://${FLINK_JM_ADDR}/jars/upload" >upload_log; then
    if jar_id=$(grep -Po 'flink-web-upload\/(.*?).jar' <upload_log | cut -d "/" -f 2); then
      cat upload_log
      echo -e "\n\nSucceed to upload the jar.\n"
      return 0
    else
      cat upload_log
      echo -e "\n\nFailed to upload the jar."
    fi
  else
    echo -e "\nFailed request to upload."
  fi

  return 1
}

function submit_job() {
  echo "curl -sS -d \"{\"jobId\":\"${FLINK_JOB_ID}\"}\" -H \"Content-Type: application/json\" -X POST http://${FLINK_JM_ADDR}/jars/${jar_id}/run"
  if curl -sS -d "{\"jobId\":\"${FLINK_JOB_ID}\"}" \
    -H "Content-Type: application/json" \
    -X POST "http://${FLINK_JM_ADDR}/jars/${jar_id}/run" >job_submit_log; then
    if grep -e "\"jobid\":\"${FLINK_JOB_ID}\"" <job_submit_log; then
      echo -e "\nSucceed to submit job."
      return 0
    else
      cat job_submit_log
      echo -e "\n\nFailed to submit job."
    fi
  else
    echo -e "\nFailed request to submit job."
  fi

  return 1
}

function main() {
  local jar_id

  echo -e "* Checking if the job has already been submitted...\n"
  if ! { \
    check_job_not_submitted \
    && upload_jar \
    && submit_job; \
  }; then
    echo -e "\nStop submitting the job."
    return 1
  fi

  echo -e "\n* Finished to submit job."
}

main "$@"
`
