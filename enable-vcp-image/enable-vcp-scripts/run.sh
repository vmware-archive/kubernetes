#!/bin/bash
source $(dirname "$0")/exit_codes.sh

[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$NODE_NAME" ] && { echo "[ERROR] NODE_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$POD_ROLE" ] && { echo "[ERROR] POD_ROLE is not set"; exit $ERROR_POD_ENV_VALIDATION; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME

if [ "$POD_ROLE" == "MANAGER" ]; then
    echo "Running Manager Role"
    /opt/enable-vcp-scripts/manager_pod.sh
elif [ "$POD_ROLE" == "DAEMON" ]; then
    echo "Running Daemon Role"
    /opt/enable-vcp-scripts/daemonset_pod.sh
else
    echo "[ERROR] Invalid Role"; 
    exit $ERROR_INVALID_POD_ROLE;
fi
sleep infinity