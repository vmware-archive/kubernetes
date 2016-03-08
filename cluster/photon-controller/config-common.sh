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

SSH_OPTS="-oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -oLogLevel=ERROR"

##########################################################
#
# Parameters needed for interacting with Photon Controller
#
##########################################################

# Pre-created tenant for Kubernetes to use
PHOTON_TENANT=kube

# Pre-created project in PHOTON_TENANT for Kubernetes to use
PHOTON_PROJECT=kube

# Pre-created VM flavor for Kubernetes to use
PHOTON_VM_FLAVOR=kube-vm

# Pre-created disk flavor for Kubernetes to use
PHOTON_DISK_FLAVOR=kube-disk

# Pre-created Debian 8 image with kube user uploaded to Photon Controller
PHOTON_IMAGE_ID="image-id-goes-here"
