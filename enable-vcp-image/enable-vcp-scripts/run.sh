#!/bin/bash
[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit 1; }
[ -z "$NODE_NAME" ] && { echo "[ERROR] NODE_NAME is not set"; exit 1; }
[ -z "$POD_ROLE" ] && { echo "[ERROR] POD_ROLE is not set"; exit 1; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME

if [ $POD_ROLE = "MANAGER" ]; then
    echo "Running Manager Role"
    /opt/enable-vcp-scripts/manager_pod.sh
elif [ $POD_ROLE = "DAEMON" ]; then
    echo "Running Daemon Role"
    /opt/enable-vcp-scripts/daemonset_pod.sh
else
    echo "[ERROR] Invalid Role"; exit 1;
fi
sleep infinity