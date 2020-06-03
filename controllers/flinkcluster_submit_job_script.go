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

JOB_MANAGER="$2"

function list_jobs() {
	for i in {1..10}; do
		if /opt/flink/bin/flink list -a --jobmanager "${JOB_MANAGER}" 2>&1; then
			return 0
		else
			sleep 5
		fi
	done

	echo "Failed to list jobs." >&2
	return 1
}

function check_existing_jobs() {
	echo "Checking existing jobs..."
	list_jobs
	if list_jobs | grep -e "(SCHEDULED)" -e "(CREATED)" -e "(SUSPENDED)" -e "(FINISHED)" -e "(FAILED)" -e "(CANCELED)" \
		-e "(RUNNING)" -e "(RESTARTING)" -e "(CANCELLING)" -e "(FAILING)" -e "(RECONCILING)"; then
		echo "Found an existing job, skip resubmitting..."
		return 0
	fi
	return 1
}

function submit_job() {
	echo -e "\nSubmitting job..."
	echo "/opt/flink/bin/flink run $@"
	/opt/flink/bin/flink run "$@"
}

function wait_for_job() {
	while true; do
		echo -e "\nWaiting for job to finish..."
		list_jobs

		# Find active job first.
		# If the current job is restarted by the operator, there will be records of past stopped jobs.
		# TODO: It needs to be improved to determine the job state with the submitted job id.
		if list_jobs | grep -e "(SCHEDULED)" -e "(CREATED)" -e "(SUSPENDED)" \
			-e "(RUNNING)" -e "(RESTARTING)" -e "(CANCELLING)" -e "(FAILING)" -e "(RECONCILING)"; then
			echo -e "\nFound an active job."
		else
			if list_jobs | grep "(FINISHED)"; then
				echo -e "\nJob has completed successfully, exiting 0"
				return 0
			fi
			if list_jobs | grep "(FAILED)"; then
				echo -e "\nJob failed, exiting 1"
				return 1
			fi
			if list_jobs | grep "(CANCELED)"; then
				echo -e "\nJob has been cancelled, exiting 2"
				return 2
			fi
			echo -e "\nUnknown job state, check it again in the next iteration."
		fi
		sleep 30
	done
}

function main() {
	if ! check_existing_jobs "$@"; then
		submit_job "$@"
	fi

	wait_for_job "$@"
}

main "$@"
`
