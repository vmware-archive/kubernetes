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

##########################################################
#
# Common parameters for Kubernetes
#
##########################################################

# Default number of minion nodes to make. You can change this as needed
NUM_NODES=1

# Range of IPs assigned to pods
NODE_IP_RANGES="10.244.0.0/16"

# IPs used by Kubernetes master
MASTER_IP_RANGE="${MASTER_IP_RANGE:-10.246.0.0/24}"

# Range of IPs assigned by Kubernetes to services
SERVICE_CLUSTER_IP_RANGE="10.244.240.0/20"

##########################################################
#
# Advanced parameters for Kubernetes
#
##########################################################

INSTANCE_PREFIX=kubernetes

# Naming scheme for nodes
MASTER_NAME="${INSTANCE_PREFIX}-master"
NODE_NAMES=($(eval echo ${INSTANCE_PREFIX}-minion-{1..${NUM_NODES}}))

# Name and password for the VM user
# The password is only used for the intial setup, and in 
# the future it will be eliminated
VM_USER=kube
VM_PASSWORD=kube

# Optional: Enable node logging.
ENABLE_NODE_LOGGING=false
LOGGING_DESTINATION=elasticsearch

# SSH options for how we connect to the Kubernetes VMs
SSH_OPTS="-oStrictHostKeyChecking=no -oLogLevel=ERROR"

# Optional: When set to true, Elasticsearch and Kibana will be setup as part of the cluster bring up.
ENABLE_CLUSTER_LOGGING=false
ELASTICSEARCH_LOGGING_REPLICAS=1

# Optional: Cluster monitoring to setup as part of the cluster bring up:
#   none     - No cluster monitoring setup 
#   influxdb - Heapster, InfluxDB, and Grafana 
#   google   - Heapster, Google Cloud Monitoring, and Google Cloud Logging
ENABLE_CLUSTER_MONITORING="${KUBE_ENABLE_CLUSTER_MONITORING:-influxdb}"

# Optional: Install cluster DNS.
ENABLE_CLUSTER_DNS="${KUBE_ENABLE_CLUSTER_DNS:-true}"
DNS_SERVER_IP="10.244.240.240"
DNS_DOMAIN="cluster.local"
DNS_REPLICAS=1

# Optional: Install Kubernetes UI
# This is currently disabled because it's not fully working
ENABLE_CLUSTER_UI=false

# Optional: if set to true, kube-up will configure the cluster to run e2e tests.
E2E_STORAGE_TEST_ENVIRONMENT=${KUBE_E2E_STORAGE_TEST_ENVIRONMENT:-false}
