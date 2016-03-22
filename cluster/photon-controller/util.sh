#!/bin/sh

# Copyright 2016 The Kubernetes Authors All rights reserved.
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

PHOTON="photon -n"

#####################################################################
#
# Public API
#
#####################################################################

#
# detect-master will query Photon Controller for the Kubernetes master.
# It assumes that the VM name for the master is unique.
# It will set KUBE_MASTER_ID to be the VM ID of the master
# It will set KUBE_MASTER_IP to be the IP address of the master
# If the silent parameter is passed, it will not print when the master
# is found: this is used internally just to find the MASTER
#
function detect-master {
    local silent=${1:-""}
    KUBE_MASTER=${MASTER_NAME}
    KUBE_MASTER_ID=${KUBE_MASTER_ID:-""}
    KUBE_MASTER_IP=${KUBE_MASTER_IP:-""}

    # We don't want silent failure: we check for failure
    set +o pipefail
    if [ -z ${KUBE_MASTER_ID} ]; then
        KUBE_MASTER_ID=$($PHOTON vm list | grep "\tkubernetes-master\t" | awk '{print $1}')
    fi
    if [ -z ${KUBE_MASTER_ID} ]; then
        kube::log::error "Could not find Kubernetes master node ID. Make sure you've launched a cluster with kube-up.sh"
        exit 1
    fi

    if [ -z "${KUBE_MASTER_IP-}" ]; then
        # Make sure to ignore lines where it's not attached to a portgroup
        # Make sure to ignore lines that have a network interface but no address
        KUBE_MASTER_IP=$($PHOTON vm networks ${KUBE_MASTER_ID} | grep -v "^-" | grep -E '\d+\.\d+\.\d+\.\d+' | head -1 | awk -Ft '{print $3}')
    fi
    if [ -z "${KUBE_MASTER_IP-}" ]; then
        kube::log::error "Could not find Kubernetes master node IP. Make sure you've launched a cluster with 'kube-up.sh'" >&2
        exit 1
    fi
    if [ -z $silent ]; then
        kube::log::status "Master: $KUBE_MASTER ($KUBE_MASTER_IP)"
    fi
    # Reset default set in common.sh
    set -o pipefail
}

#
# detect-nodes will query Photon Controller for the Kubernetes minions
# It assumes that the VM name for the minions are unique.
# It assumes that NODE_NAMES has been set
# It will set KUBE_NODE_IP_ADDRESSES to be the VM IPs of the minions
# It will set the KUBE_NODE_IDS to be the VM IDs of the minions
# If the silent parameter is passed, it will not print when the nodes
# are found: this is used internally just to find the MASTER
#
function detect-nodes {
    local silent=${1:-""}
    local failure=0

    KUBE_NODE_IP_ADDRESSES=()
    KUBE_NODE_IDS=()
    # We don't want silent failure: we check for failure
    set +o pipefail
    for (( i=0; i<${#NODE_NAMES[@]}; i++)); do

        local node_id=$($PHOTON vm list | grep "\t${NODE_NAMES[$i]}\t" | awk '{print $1}')
        if [ -z ${node_id} ]; then
            kube::log::error "Could not find ${NODE_NAMES[$i]}"
            failure=1
        fi
        KUBE_NODE_IDS+=("${node_id}")

        # Make sure to ignore lines where it's not attached to a portgroup
        # Make sure to ignore lines that have a network interface but no address
        local node_ip=$($PHOTON vm networks ${node_id} | grep -v "^-" | grep -E '\d+\.\d+\.\d+\.\d+'| head -1 | awk -Ft '{print $3}')
        KUBE_NODE_IP_ADDRESSES+=("${node_ip}")

        if [ -z $silent ]; then
            kube::log::status "Minion: ${NODE_NAMES[$i]} (${KUBE_NODE_IP_ADDRESSES[$i]})"
        fi
    done

    if [ $failure -ne 0 ]; then
        exit 1
    fi
    # Reset default set in common.sh
    set -o pipefail
}

# Get node names if they are not static.
function detect-node-names {
	echo "TODO: detect-node-names" 1>&2
}

#
# Verifies that this computer has sufficient software installed
# so that it can run the rest of the script.
#
function verify-prereqs {
    verify-cmd-in-path photon
    verify-cmd-in-path ssh
    verify-cmd-in-path scp
    verify-cmd-in-path ssh-add
    verify-cmd-in-path sshpass
}

#
# The entry point for bringing up a Kubernetes cluster
#
function kube-up {
    verify-prereqs
    verify-ssh-prereqs
    verify-photon-config
    ensure-temp-dir
    find-release-tars
    find-image-id

    gen-master-start
    create-master-vm
    install-salt-on-master

    gen-node-start
    install-nodes

    detect-nodes -s

    install-kubernetes-on-master
    install-kubernetes-on-nodes

    wait-master-api
    wait-minion-apis

    setup-pod-routes

    copy-kube-certs
    kube::log::status "Creating kubeconfig..."
    create-kubeconfig
}

# Delete a kubernetes cluster
function kube-down {
    detect-master
    detect-nodes

    pc-delete-vm $KUBE_MASTER $KUBE_MASTER_ID
    for (( node=0; node<${#KUBE_NODE_IDS[@]}; node++)); do
        pc-delete-vm ${NODE_NAMES[$node]} ${KUBE_NODE_IDS[$node]}
    done
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

#####################################################################
#
# Internal functions
#
#####################################################################

#
# Uses Photon Controller to make a VM
# Takes two parameters:
#   - The name of the VM (Assumed to be unique)
#   - The name of the flavor to create the VM (Assumed to be unique)
# It assumes that the variables in config-common.sh (PHOTON_TENANT, etc)
# are set correctly.
# When it completes, it sets two environment variables for use by the
# caller: _VM_ID (the ID of the created VM) and _VM_IP (the IP address
# of the created VM)
#
function pc-create-vm {
    local vm_name="$1"
    local vm_flavor="$2"
    local rc=0
    local i=0

    # Create the VM
    tenant_args="--tenant ${PHOTON_TENANT} --project ${PHOTON_PROJECT}"
    vm_args="--name ${vm_name} --image ${PHOTON_IMAGE_ID} --flavor ${vm_flavor}"
    disk_args="disk-1 ${PHOTON_DISK_FLAVOR} boot=true"

    rc=0
    _VM_ID=$($PHOTON vm create ${tenant_args} ${vm_args} --disks "${disk_args}" 2>&1) || rc=$?
    if [ $rc -ne 0 ]; then
        kube::log::error "Failed to create VM. Error output:"
        echo ${_VM_ID}
        exit 1
    fi
    kube::log::status "Created VM ${vm_name}: ${_VM_ID}"

    # Start the VM
    run-cmd "$PHOTON vm start ${_VM_ID}"
    kube::log::status "Started VM ${vm_name}, waiting for network address..."

    # Wait for the VM to be started and connected to the network
    have_network=0
    for i in $(seq 120); do
        # photon -n vm networks print several fields:
        # NETWORK MAC IP GATEWAY CONNECTED?
        # We wait until CONNECTED is True
        rc=0
        networks=$($PHOTON vm networks ${_VM_ID}) || rc=$?
        if [ $rc -ne 0 ]; then
            kube::log::error "'$PHOTON vm networks ${_VM_ID}' failed. Error output: "
            echo $networks
        fi
        networks=$(echo "$networks" | grep True) || rc=$?
        if [ $rc -eq 0 ]; then
            have_network=1
            break;
        fi
        sleep 1
    done

    # Fail if the VM didn't come up
    if [ $have_network -eq 0 ]; then
        kube::log::error "VM ${vm_name} failed to start up: no IP was found"
        exit 1
    fi

    # Find the IP address of the VM
    _VM_IP=$(PHOTON -n vm networks ${_VM_ID} | head -1 | awk -Ft '{print $3}')
    kube::log::status "VM ${vm_name} has IP: ${_VM_IP}"

    # Find this user's ssh public keys, if any.
    # We can't use run-cmd, because it uses $(), which removes newlines from output
    rc=0
    auth_key_file="${KUBE_TEMP}/${vm_name}-authorized_keys"
    ssh-add -L > ${auth_key_file}

    # And copy them to the VM
    run-ssh-cmd -p ${_VM_IP} "mkdir /home/kube/.ssh"
    run-ssh-cmd -p ${_VM_IP} "chmod 700 /home/kube/.ssh"
    copy-file-to-vm -p ${_VM_IP} ${auth_key_file} "/home/kube/.ssh/authorized_keys"
    run-ssh-cmd -p ${_VM_IP} "chmod 600 /home/kube/.ssh/authorized_keys"
    rm -f ${auth_key_file}
}

#
# Delete one of our VMs
# If it is STARTED, it will be stopped first.
#
function pc-delete-vm {
    local vm_name="$1"
    local vm_id="$2"
    local rc=0

    kube::log::status "Deleting VM ${vm_name}"
    $PHOTON vm show $vm_id | head -1 | grep STARTED > /dev/null 2>&1 || rc=$?
    if [ $rc -eq 0 ]; then
        $PHOTON vm stop $vm_id > /dev/null 2>&1 || rc=$?
        if [ $rc -ne 0 ]; then
            kube::log::error "Error: could not stop ${vm_name} ($vm_id)"
            kube::log::error "Please investigate and stop manually"
            return
        fi
    fi

    rc=0
    $PHOTON vm delete $vm_id > /dev/null 2>&1 || rc=$?
    if [ $rc -ne 0 ]; then
        kube::log::error "Error: could not delete ${vm_name} ($vm_id)"
        kube::log::error "Please investigate and delete manually"
    fi
}

#
# Looks for the image named PHOTON_IMAGE
# Set PHOTON_IMAGE_ID to be the id of that image.
# We currently assume there is exactly one image with name
#
function find-image-id {
    local rc=0
    PHOTON_IMAGE_ID=$($PHOTON image list | head -1 | grep "\t${PHOTON_IMAGE}\t" | awk '{print $1}')
    if [ $rc -ne 0 ]; then
        kube::log::error "Cannot find image \"${PHOTON_IMAGE}\""
        fail=1
    fi
}

#
# Generate a script used to install salt on the master
# It is placed into $KUBE_TEMP/master-start.sh
#
function gen-master-start {
    load-or-gen-kube-basicauth
    python "${KUBE_ROOT}/third_party/htpasswd/htpasswd.py" \
        -b -c "${KUBE_TEMP}/htpasswd" "$KUBE_USER" "$KUBE_PASSWORD"
    local htpasswd
    htpasswd=$(cat "${KUBE_TEMP}/htpasswd")

    (
        echo "#! /bin/bash"
        echo "readonly MY_NAME=${MASTER_NAME}"
        grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/hostname.sh"
        echo "cd /home/kube/cache/kubernetes-install"
        echo "readonly MASTER_NAME='${MASTER_NAME}'"
        echo "readonly MASTER_IP_RANGE='${MASTER_IP_RANGE}'"
        echo "readonly INSTANCE_PREFIX='${INSTANCE_PREFIX}'"
        echo "readonly NODE_INSTANCE_PREFIX='${INSTANCE_PREFIX}-node'"
        echo "readonly NODE_IP_RANGES='${NODE_IP_RANGES}'"
        echo "readonly SERVICE_CLUSTER_IP_RANGE='${SERVICE_CLUSTER_IP_RANGE}'"
        echo "readonly ENABLE_NODE_LOGGING='${ENABLE_NODE_LOGGING:-false}'"
        echo "readonly LOGGING_DESTINATION='${LOGGING_DESTINATION:-}'"
        echo "readonly ENABLE_CLUSTER_DNS='${ENABLE_CLUSTER_DNS:-false}'"
        echo "readonly DNS_SERVER_IP='${DNS_SERVER_IP:-}'"
        echo "readonly DNS_DOMAIN='${DNS_DOMAIN:-}'"
        echo "readonly KUBE_USER='${KUBE_USER:-}'"
        echo "readonly KUBE_PASSWORD='${KUBE_PASSWORD:-}'"
        echo "readonly SERVER_BINARY_TAR='${SERVER_BINARY_TAR##*/}'"
        echo "readonly SALT_TAR='${SALT_TAR##*/}'"
        echo "readonly MASTER_HTPASSWD='${htpasswd}'"
        echo "readonly E2E_STORAGE_TEST_ENVIRONMENT='${E2E_STORAGE_TEST_ENVIRONMENT:-}'"
        grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/create-dynamic-salt-files.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/install-release.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/salt-master.sh"
    ) > "${KUBE_TEMP}/master-start.sh"
}

#
# Generate the scripts for each minion to install salt
#
function gen-node-start {
    local i
    for (( i=0; i<${#NODE_NAMES[@]}; i++)); do
        (
            echo "#! /bin/bash"
            echo "readonly MY_NAME=${NODE_NAMES[$i]}"
            grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/hostname.sh"
            echo "KUBE_MASTER=${KUBE_MASTER}"
            echo "KUBE_MASTER_IP=${KUBE_MASTER_IP}"
            echo "NODE_IP_RANGE=$NODE_IP_RANGES"
            grep -v "^#" "${KUBE_ROOT}/cluster/vsphere/templates/salt-minion.sh"
        ) > "${KUBE_TEMP}/node-start-${i}.sh"
  done
}

#
# Create a script that will run on the Kubernetes master and will run salt
# to configure the master. We make it a script instead of just running a
# single ssh command so that we can get loging.
#
function gen-master-salt {
     (
        echo '#!/bin/bash'
        echo 'echo $(date) >> /tmp/master-salt.log'
        echo "salt kubernetes-master state.highstate -t 30 --no-color > /tmp/master-salt.out"
        echo 'grep -E "Failed:[[:space:]]+0" /tmp/master-salt.out'
        echo 'success=$?'
        echo 'cat /tmp/master-salt.out >> /tmp/master-salt.log'
        echo 'exit $success'
    ) > ${KUBE_TEMP}/salt-master.sh
}

#
# Create scripts that will be run on the Kubernets master. Each of these
# will invoke salt to configure one of the minions
#
function gen-node-salt {
    local i
    for (( i=0; i<${#NODE_NAMES[@]}; i++)); do
        (
            echo '#!/bin/bash'
            echo "echo \$(date) >> /tmp/${NODE_NAMES[$i]}-salt.log"
            echo "salt ${NODE_NAMES[$i]} state.highstate -t 30 --no-color > /tmp/${NODE_NAMES[$i]}-salt.out"
            echo "grep -E \"Failed:[[:space:]]+0\" /tmp/${NODE_NAMES[$i]}-salt.out"
            echo 'success=$?'
            echo "cat /tmp/${NODE_NAMES[$i]}-salt.out >> /tmp/${NODE_NAMES[$i]}-salt.log"
            echo 'exit $success'
        ) > "${KUBE_TEMP}/${NODE_NAMES[$i]}-salt.sh"
    done
}

function create-master-vm {
    kube::log::status "Starting master VM..."
    pc-create-vm ${MASTER_NAME} ${PHOTON_MASTER_FLAVOR}
    KUBE_MASTER=${MASTER_NAME}
    KUBE_MASTER_ID=$_VM_ID
    KUBE_MASTER_IP=$_VM_IP
}

function install-salt-on-master {
    kube::log::status "Installing salt on master..."
    upload-server-tars ${MASTER_NAME} ${KUBE_MASTER_IP}
    run-script-remotely ${KUBE_MASTER_IP} ${KUBE_TEMP}/master-start.sh
}

function install-nodes {
    kube::log::status "Creating minions and installing salt on them..."

    # Start each of the VMs in parallel
    local node
    for (( node=0; node<${#NODE_NAMES[@]}; node++)); do
    (
        pc-create-vm ${NODE_NAMES[$node]} ${PHOTON_NODE_FLAVOR}
        run-script-remotely ${_VM_IP} "${KUBE_TEMP}/node-start-${node}.sh"
    ) &
    done

    # Wait for the node VM startups to complete
    local fail=0
    local job
    for job in $(jobs -p); do
        wait "${job}" || fail=$((fail + 1))
    done
    if (( $fail != 0 )); then
        kube::log::error "Failed to start ${fail}/${NUM_NODES} minions"
        exit 1
    fi
}

function install-kubernetes-on-master {
    # Wait until salt-master is running: it may take a bit
    try-until-success-ssh ${KUBE_MASTER_IP} \
        "Waiting for salt-master to start on ${KUBE_MASTER}" \
        "pgrep salt-master"
    gen-master-salt
    copy-file-to-vm -p ${_VM_IP} ${KUBE_TEMP}/salt-master.sh "/tmp/master-salt.sh"
    try-until-success-ssh ${KUBE_MASTER_IP} \
        "Installing Kubernetes on ${KUBE_MASTER} via salt" \
        "sudo /bin/bash /tmp/master-salt.sh"
}

function install-kubernetes-on-nodes {
    gen-node-salt

    # Run in parallel to bring up the cluster faster
    # TODO: Batch this so that we run up to N in parallel, so
    # we don't overload this machine
    local node
    for (( node=0; node<${#NODE_NAMES[@]}; node++)); do
    (
        copy-file-to-vm -p ${_VM_IP} ${KUBE_TEMP}/${NODE_NAMES[$node]}-salt.sh "/tmp/${NODE_NAMES[$node]}-salt.sh"
        try-until-success-ssh ${KUBE_NODE_IP_ADDRESSES[$node]} \
            "Waiting for salt-master to start on ${NODE_NAMES[$node]}" \
            "pgrep salt-minion"
        try-until-success-ssh ${KUBE_MASTER_IP} \
            "Installing Kubernetes on ${NODE_NAMES[$node]} via salt" \
            "sudo /bin/bash /tmp/${NODE_NAMES[$node]}-salt.sh"
    ) &
    done

    # Wait for the Kubernetes installations to complete
    local fail=0
    local job
    for job in $(jobs -p); do
        wait "${job}" || fail=$((fail + 1))
    done
    if (( $fail != 0 )); then
        kube::log::error "Failed to start install Kubernetes on ${fail} out of ${NUM_NODES} minions"
        exit 1
    fi
}

#
# Upload the Kubernetes tarballs to the master
#
function upload-server-tars {
    vm_name=$1
    vm_ip=$2

    run-ssh-cmd ${vm_ip} "mkdir -p /home/kube/cache/kubernetes-install"

    local tar
    for tar in "${SERVER_BINARY_TAR}" "${SALT_TAR}"; do
        local base_tar
        base_tar=$(basename $tar)
        kube::log::status "Uploading ${base_tar} to ${vm_name}..."
        copy-file-to-vm ${vm_ip} "${tar}" "/home/kube/cache/kubernetes-install/${tar##*/}"
    done
}

#
# Wait for the Kubernets healthz API to be responsive on the master
#
function wait-master-api {
    local curl_creds="--insecure --user ${KUBE_USER}:${KUBE_PASSWORD}"
    local curl_output="--fail --output /dev/null --silent"
    local curl_net="--max-time 1"

    try-until-success "Waiting for Kubernetes API on ${KUBE_MASTER}" \
        "curl ${curl_creds} ${curl_output} ${curl_net} https://${KUBE_MASTER_IP}/healthz"
}

#
# Wait for the Kubernetes healthz API to be responsive on each minion
#
function wait-minion-apis {
    local curl_output="--fail --output /dev/null --silent"
    local curl_net="--max-time 1"

    for (( i=0; i<${#NODE_NAMES[@]}; i++)); do
        try-until-success "Waiting for Kubernetes API on ${NODE_NAMES[$i]}..." \
            "curl ${curl_output} ${curl_net} http://${KUBE_NODE_IP_ADDRESSES[$i]}:10250/healthz"
    done
}

#
# Configure the minions so the pods can communicate
# Each minion will have a bridge named cbr0 for the NODE_IP_RANGES
# defined in config-default.sh. This finds the IP address (assigned
# by Kubernetes) to minion and configures routes so they can communicate
#
function setup-pod-routes {
    local node

    KUBE_NODE_BRIDGE_NETWORK=()
    for (( node=0; node<${#NODE_NAMES[@]}; node++)); do

        # This happens in two steps (wait for an address, wait for a non 172.x.x.x address)
        # because it's both simpler and more clear what's happening.
        try-until-success-ssh ${KUBE_NODE_IP_ADDRESSES[$node]} \
            "Waiting for cbr0 bridge on ${NODE_NAMES[$node]} to have an address"  \
            'sudo ifconfig cbr0  | grep -oP "inet addr:\K\S+"'

        try-until-success-ssh ${KUBE_NODE_IP_ADDRESSES[$node]} \
            "Waiting for cbr0 bridge on ${NODE_NAMES[$node]} to have correct address"  \
            'sudo ifconfig cbr0  | grep -oP "inet addr:\K\S+" | grep -v  "^172."'

        run-ssh-cmd ${KUBE_NODE_IP_ADDRESSES[$node]} 'sudo ip route show | grep -E "dev cbr0" | cut -d " " -f1'
        KUBE_NODE_BRIDGE_NETWORK+=($_OUTPUT)
        kube::log::status "cbr0 on ${NODE_NAMES[$node]} is ${_OUTPUT}"
    done

    local i
    local j
    for (( i=0; i<${#NODE_NAMES[@]}; i++)); do
        kube::log::status "Configuring pod routes on ${NODE_NAMES[$i]}..."
        for (( j=0; j<${#NODE_NAMES[@]}; j++)); do
            if [[ $i != $j ]]; then
                run-ssh-cmd ${KUBE_NODE_IP_ADDRESSES[$i]} "sudo route add -net ${KUBE_NODE_BRIDGE_NETWORK[$j]} gw ${KUBE_NODE_IP_ADDRESSES[$j]}"
            fi
        done
    done
}

#
# Copy the certificate/key from the Kubernetes master
# These are used to create the kubeconfig file, which allows
# users to use kubectl easily
#
function copy-kube-certs {
    local cert="kubecfg.crt"
    local key="kubecfg.key"
    local ca="ca.crt"
    local cert_dir="/srv/kubernetes"

    kube::log::status "Copying credentials from ${KUBE_MASTER}"

    KUBE_CERT="${KUBE_TEMP}/${cert}"
    KUBE_KEY="${KUBE_TEMP}/${key}"
    CA_CERT="${KUBE_TEMP}/${ca}"
    CONTEXT="photon-${INSTANCE_PREFIX}"

    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 644 ${cert_dir}/${cert}"
    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 644 ${cert_dir}/${key}"
    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 644 ${cert_dir}/${ca}"

    copy-file-from-vm ${KUBE_MASTER_IP} ${cert_dir}/${cert} ${KUBE_CERT}
    copy-file-from-vm ${KUBE_MASTER_IP} ${cert_dir}/${key}  ${KUBE_KEY}
    copy-file-from-vm ${KUBE_MASTER_IP} ${cert_dir}/${ca}   ${CA_CERT}

    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 600 ${cert_dir}/${cert}"
    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 600 ${cert_dir}/${key}"
    run-ssh-cmd ${KUBE_MASTER_IP} "sudo chmod 600 ${cert_dir}/${ca}"
}

#
# Copies a script to a VM and runs it
# Parameters:
#   - IP of VM
#   - Path to local file
#
function run-script-remotely {
    local vm_ip=$1
    local local_file="$2"
    local base_file=$(basename ${local_file})
    local remote_file="/tmp/$base_file"

    copy-file-to-vm ${vm_ip} ${local_file} ${remote_file}
    run-ssh-cmd ${vm_ip} "chmod 700 ${remote_file}"
    run-ssh-cmd ${vm_ip} "nohup sudo ${remote_file} < /dev/null 1> ${remote_file}.out 2>&1 &"
}

#
# Runs an command on a VM using ssh
# If -p is the first parameter, it uses sshpass to login. This is only done
# during bootstrapping.
# Parameters:
#   - (optional) -p to use sshpass
#   - (optional) -i to ignore failure
#   - IP address of the VM
#   - Command to run
# Assumes environment variables:
#   - VM_USER
#   - VM_PASSWORD
#   - SSH_OPTS
#
function run-ssh-cmd {
    local use_sshpass=0
    local ignore_failure=""
    if [ "$1" = "-p" ]; then
        use_sshpass=1
        shift
    fi
    if [ "$1" = "-i" ]; then
        ignore_failure="-i"
        shift
    fi

    local vm_ip=$1
    shift

    if [ $use_sshpass -eq 1 ]; then
        run-cmd ${ignore_failure} "sshpass -p $VM_PASSWORD ssh $SSH_OPTS $VM_USER@${vm_ip} $@"
    else
        run-cmd ${ignore_failure} "ssh $SSH_OPTS $VM_USER@${vm_ip} $@"
    fi
}

#
# Uses scp to copy file to VM
# If -p is the first parameter, uses sshpass to login. This is only done
# during bootstrapping.
# Parameters:
#   - (optional) -p to use sshpass
#   - IP address of the VM
#   - Path to local file
#   - Path to remote file
# Assumes environment variables:
#   - VM_USER
#   - VM_PASSWORD
#   - SSH_OPTS
#
function copy-file-to-vm {
    use_sshpass=0
    if [ "$1" = "-p" ]; then
        use_sshpass=1
        shift
    fi
    local vm_ip=$1
    local local_file=$2
    local remote_file=$3

    if [ $use_sshpass -eq 1 ]; then
        run-cmd "sshpass -p $VM_PASSWORD scp $SSH_OPTS $local_file $VM_USER@${vm_ip}:$remote_file"
    else
        run-cmd "scp $SSH_OPTS $local_file $VM_USER@${vm_ip}:$remote_file"
    fi
}

function copy-file-from-vm {
    local vm_ip=$1
    local remote_file=$2
    local local_file=$3

    run-cmd "scp $SSH_OPTS $VM_USER@${vm_ip}:$remote_file $local_file"
}

#
# Run a command, print nice error output
# Used by copy-file-to-vm and run-ssh-cmd
#
function run-cmd {
    local rc=0
    local ignore_failure=""
    if [ "$1" = "-i" ]; then
        ignore_failure=$1
        shift
    fi

    local cmd=$@
    local output
    output=$($@ 2>&1) || rc=$?
    if [ $rc -ne 0 ]; then
        if [ -z "$ignore_failure" ]; then
            kube::log::error "Failed to run command: $cmd Output:"
            echo $output
            exit 1
        fi
    fi
    _OUTPUT=$output
    return $rc
}

#
# After the initial VM setup, we use SSH with keys to access the VMs
# This requires an SSH agent, so we verify that it's running
#
function verify-ssh-prereqs {
    kube::log::status "Validating SSH configuration..."
    local rc

    rc=0
    ssh-add -L 1> /dev/null 2> /dev/null || rc=$?
    # "Could not open a connection to your authentication agent."
    if [[ "${rc}" -eq 2 ]]; then
        # ssh agent wasn't running, so start it and ensure we stop it
        eval "$(ssh-agent)" > /dev/null
        trap-add "kill ${SSH_AGENT_PID}" EXIT
    fi

    rc=0
    ssh-add -L 1> /dev/null 2> /dev/null || rc=$?
    # "The agent has no identities."
    if [[ "${rc}" -eq 1 ]]; then
    # Try adding one of the default identities, with or without passphrase.
        ssh-add || true
    fi

    # Expect at least one identity to be available.
    if ! ssh-add -L 1> /dev/null 2> /dev/null; then
        kube::log::error "Could not find or add an SSH identity."
        kube::log::error "Please start ssh-agent, add your identity, and retry."
        exit 1
    fi
}

#
# Verify that Photon Controller has been configured in the way we expect. Specifically
# - Have the flavors been created?
# - Has the image been uploaded?
# TODO: Check the tenant and project as well.
function verify-photon-config {
    kube::log::status "Validating Photon configuration..."

    fail=0
    rc=0
    $PHOTON flavor list | awk -Ft '{print $2}' | grep "^${PHOTON_MASTER_FLAVOR}$" > /dev/null 2>&1 || rc=$?
    if [ $rc -ne 0 ]; then
        kube::log::error "Cannot find flavor named ${PHOTON_MASTER_FLAVOR}"
        fail=1
    fi

    if [ "${PHOTON_MASTER_FLAVOR}" != "${PHOTON_NODE_FLAVOR}" ]; then
        rc=0
        $PHOTON flavor list | awk -Ft '{print $2}' | grep "^${PHOTON_NODE_FLAVOR}$" > /dev/null 2>&1 || rc=$?
        if [ $rc -ne 0 ]; then
            kube::log::error "Cannot find flavor named ${PHOTON_NODE_FLAVOR}"
            fail=1
        fi
    fi

    rc=0
    $PHOTON image list | grep "\t${PHOTON_IMAGE}\t"  > /dev/null 2>&1 || rc=$?
    if [ $rc -ne 0 ]; then
        kube::log::error "Cannot find image \"${PHOTON_IMAGE}\""
        fail=1
    fi

    if [ $fail -eq 1 ]; then
        exit 1
    fi
}

#
# Verifies that a given command is in the PATH
#
function verify-cmd-in-path {
    cmd=$1
    which ${cmd} >/dev/null || {
        kube::log::error "Can't find ${cmd} in PATH, please install and retry."
        exit 1
    }
}

#
# Checks that KUBE_TEMP is set, or sets it
# If it sets it, it also creates the temporary directory
# and sets up a trap so that we delete it when we exit
#
function ensure-temp-dir {
  if [[ -z ${KUBE_TEMP-} ]]; then
    KUBE_TEMP=$(mktemp -d -t kubernetes.XXXXXX)
    trap-add 'rm -rf "${KUBE_TEMP}"' EXIT
  fi
}

#
# Repeatedly try a command over ssh until it succeeds or until five minutes have passed
# The timeout isn't exact, since we assume the command runs instantaneously, and
# it doesn't.
#
function try-until-success-ssh {
    local vm_ip=$1
    local cmd_description=$2
    local cmd=$3
    local timeout=600
    local sleep_time=5
    local max_attempts

    ((max_attempts=timeout/sleep_time))

    kube::log::status "$cmd_description for up to 10 minutes..."
    local attempt=0
    while true; do
        local rc=0
        run-ssh-cmd -i ${vm_ip} "$cmd" || rc=1
        if [[ $rc != 0 ]]; then
            if (( attempt == max_attempts )); then
                kube::log::error "Failed, cannot proceed: you may need to retry to log into the VM to debug"
                exit 1
            fi
        else
            break
        fi
        attempt=$((attempt+1))
        sleep $sleep_time
    done
}

function try-until-success {
    local cmd_description=$1
    local cmd=$2
    local timeout=600
    local sleep_time=5
    local max_attempts

    ((max_attempts=timeout/sleep_time))

    kube::log::status "$cmd_description for up to 10 minutes..."
    local attempt=0
    while true; do
        local rc=0
        run-cmd -i "$cmd" || rc=1
        if [[ $rc != 0 ]]; then
            if (( attempt == max_attempts )); then
                kube::log::error "Failed, cannot proceed"
                exit 1
            fi
        else
            break
        fi
        attempt=$((attempt+1))
        sleep $sleep_time
    done

}

#
# Sets up a trap handler
#
function trap-add {
  local handler="$1"
  local signal="${2-EXIT}"
  local cur

  cur="$(eval "sh -c 'echo \$3' -- $(trap -p ${signal})")"
  if [[ -n "${cur}" ]]; then
    handler="${cur}; ${handler}"
  fi

  trap "${handler}" ${signal}
}
