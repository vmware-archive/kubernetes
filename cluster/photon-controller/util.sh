#!/bin/bash

# Copyright 2014 The Kubernetes Authors All rights reserved.
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

# A library of helper functions that each provider hosting Kubernetes must implement to use cluster/kube-*.sh scripts.

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${KUBE_ROOT}/cluster/photon-controller/config-common.sh"
source "${KUBE_ROOT}/cluster/photon-controller/${KUBE_CONFIG_FILE-"config-default.sh"}"
source "${KUBE_ROOT}/cluster/common.sh"

# Must ensure that the following ENV vars are set
function detect-master {
	echo "TODO: detect-master" 1>&2
}

# Get node names if they are not static.
function detect-node-names {
	echo "TODO: detect-node-names" 1>&2
}

# Get node IP addresses and store in KUBE_NODE_IP_ADDRESSES[]
function detect-nodes {
	echo "TODO: detect-nodes" 1>&2
}

# Verify prereqs on host machine
function verify-prereqs {
	echo "TODO: verify-prereqs" 1>&2
}

# Instantiate a kubernetes cluster
function kube-up {
	echo "TODO: kube-up" 1>&2
}

# Delete a kubernetes cluster
function kube-down {
	echo "TODO: kube-down" 1>&2
}

# Update a kubernetes cluster
function kube-push {
	echo "TODO: kube-push" 1>&2
}

# Prepare update a kubernetes component
function prepare-push {
	echo "TODO: prepare-push" 1>&2
}

# Update a kubernetes master
function push-master {
	echo "TODO: push-master" 1>&2
}

# Update a kubernetes node
function push-node {
	echo "TODO: push-node" 1>&2
}

# Execute prior to running tests to build a release if required for env
function test-build-release {
	echo "TODO: test-build-release" 1>&2
}

# Execute prior to running tests to initialize required structure
function test-setup {
	echo "TODO: test-setup" 1>&2
}

# Execute after running tests to perform any required clean-up
function test-teardown {
	echo "TODO: test-teardown" 1>&2
}
