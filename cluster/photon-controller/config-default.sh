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

# Default number of nodes to make. You can change this as needed
NUM_NODES=3

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

# The instance prefix is the beginning of the name given to each VM we create
# If this is changed, you can have multiple kubernetes clusters per project
# Note that even if you don't change it, each tenant/project can have its own
# Kubernetes cluster
INSTANCE_PREFIX=kubernetes

# Naming scheme for VMs (masters & nodes)
MASTER_NAME="${INSTANCE_PREFIX}-master"
NODE_NAMES=($(eval echo ${INSTANCE_PREFIX}-node-{1..${NUM_NODES}}))

# Name of the user used to configure the VM
# We use cloud-init to create the user
VM_USER=kube

# SSH options for how we connect to the Kubernetes VMs
SSH_OPTS="-oStrictHostKeyChecking=no -oLogLevel=ERROR"

# Optional: Enable node logging.
# Note: currently untested
ENABLE_NODE_LOGGING=false
LOGGING_DESTINATION=elasticsearch

# Optional: When set to true, Elasticsearch and Kibana will be setup
# Note: currently untested
ENABLE_CLUSTER_LOGGING=false
ELASTICSEARCH_LOGGING_REPLICAS=1

# Optional: Cluster monitoring to setup as part of the cluster bring up:
#   none     - No cluster monitoring setup
#   influxdb - Heapster, InfluxDB, and Grafana
#   google   - Heapster, Google Cloud Monitoring, and Google Cloud Logging
# Note: currently untested
ENABLE_CLUSTER_MONITORING="${KUBE_ENABLE_CLUSTER_MONITORING:-influxdb}"

# Optional: Install cluster DNS.
ENABLE_CLUSTER_DNS="${KUBE_ENABLE_CLUSTER_DNS:-true}"
DNS_SERVER_IP="10.244.240.240"
DNS_DOMAIN="cluster.local"
DNS_REPLICAS=1

# Optional: Install Kubernetes UI
ENABLE_CLUSTER_UI=true

# We need to configure subject alternate names (SANs) for the master's certificate
# we generate.  While users will connect via the external IP, pods (like the UI)
# will connect via the cluster IP, from the SERVICE_CLUSTER_IP_RANGE.
#
# This calculation of the service IP should work, but if you choose an
# alternate subnet, there's a small chance you'd need to modify the
# service_ip, below.  We'll choose an IP like 10.244.240.1 by taking
# the first three octets of the SERVICE_CLUSTER_IP_RANGE and tacking
# on a .1
octets=($(echo "$SERVICE_CLUSTER_IP_RANGE" | sed -e 's|/.*||' -e 's/\./ /g'))
((octets[3]+=1))
service_ip=$(echo "${octets[*]}" | sed 's/ /./g')
MASTER_EXTRA_SANS="IP:${service_ip},DNS:kubernetes,DNS:kubernetes.default,DNS:kubernetes.default.svc,DNS:kubernetes.default.svc.${DNS_DOMAIN},DNS:${MASTER_NAME}"

# Optional: if set to true, kube-up will configure the cluster to run e2e tests.
E2E_STORAGE_TEST_ENVIRONMENT=${KUBE_E2E_STORAGE_TEST_ENVIRONMENT:-false}
