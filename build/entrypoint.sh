#!/bin/bash

# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# Check whether there is a passwd entry for the container UID
myuid=$(id -u)
mygid=$(id -g)
uidentry=$(getent passwd $myuid)
if [[ -z "${uidentry}" ]] ; then
  if [[ -w /etc/passwd ]] ; then
    echo "$myuid:x:$myuid:$mygid:anonymous uid:$SPARK_HOME:/bin/false" >> /etc/passwd
  else
    echo "Container ENTRYPOINT failed to add passwd entry for anonymous UID"
  fi
fi

echo "Running /usr/bin/flink-operator, uid=${myuid}, gid=${mygid}..."
exec /sbin/tini -s -- /usr/bin/flink-operator "$@"
