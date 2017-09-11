#!/bin/bash
# shellcheck source=./exit_codes.sh
source "$(dirname "$0")"/exit_codes.sh
# shellcheck source=./common_func.sh
source "$(dirname "$0")"/common_func.sh

[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$NODE_NAME" ] && { echo "[ERROR] NODE_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$POD_ROLE" ] && { echo "[ERROR] POD_ROLE is not set"; exit $ERROR_POD_ENV_VALIDATION; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME
# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

if [ "$POD_ROLE" == "MANAGER" ]; then
    echo "Running Manager Role"
    bash /opt/enable-vcp-scripts/manager_pod.sh
elif [ "$POD_ROLE" == "DAEMON" ]; then
    echo "Running Daemon Role"
    bash -x /opt/enable-vcp-scripts/daemonset_pod.sh "$POD_NAME" "$NODE_NAME"
else
    echo "[ERROR] Invalid Role";
    exit $ERROR_INVALID_POD_ROLE;
fi

echo "[INFO] Done with all tasks. Sleeping Infinity."
# sleep infinity
python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
