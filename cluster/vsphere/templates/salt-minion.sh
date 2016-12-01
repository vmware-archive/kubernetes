#!/bin/bash

# Copyright 2014 The Kubernetes Authors.
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

# Use other Debian mirror
sed -i -e "s/http.us.debian.org/mirrors.kernel.org/" /etc/apt/sources.list

# Resolve hostname of master
if ! grep -q $KUBE_MASTER /etc/hosts; then
  echo "Adding host entry for $KUBE_MASTER"
  echo "$KUBE_MASTER_IP $KUBE_MASTER" >> /etc/hosts
fi

# Prepopulate the name of the Master
mkdir -p /etc/salt/minion.d
echo "master: $KUBE_MASTER" > /etc/salt/minion.d/master.conf

# Turn on debugging for salt-minion
# echo "DAEMON_ARGS=\"\$DAEMON_ARGS --log-file-level=debug\"" > /etc/default/salt-minion

# Configuration to initialize vsphere cloud provider
CLOUD_CONFIG=/etc/vsphere_cloud.config

cat <<EOF > $CLOUD_CONFIG
[Global]
        user = $GOVC_USERNAME
        password = $GOVC_PASSWORD
        server = $GOVC_URL
        port = $GOVC_PORT
        insecure-flag = $GOVC_INSECURE
        datacenter = $GOVC_DATACENTER
        datastore = $GOVC_DATASTORE
        region = $GOVC_REGION
        failure-domain = $GOVC_FAILUREDOMAIN

[Disk]
	scsicontrollertype = pvscsi
EOF

# Our minions will have a pool role to distinguish them from the master.
#
# Setting the "minion_ip" here causes the kubelet to use its IP for
# identification instead of its hostname.
#
cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-pool
    - kubernetes-pool-vsphere
  cloud: vsphere
  cloud_config: $CLOUD_CONFIG
EOF

# Install Salt
#
# We specify -X to avoid a race condition that can cause minion failure to
# install.  See https://github.com/saltstack/salt-bootstrap/issues/270
curl -L --connect-timeout 20 --retry 6 --retry-delay 10 https://bootstrap.saltstack.com | sh -s -- -X stable 2016.3.2
