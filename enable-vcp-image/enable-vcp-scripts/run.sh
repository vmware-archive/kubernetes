#!/bin/bash
source $(dirname "$0")/exit_codes.sh
source $(dirname "$0")/common_func.sh

[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$NODE_NAME" ] && { echo "[ERROR] NODE_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$POD_ROLE" ] && { echo "[ERROR] POD_ROLE is not set"; exit $ERROR_POD_ENV_VALIDATION; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME
# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

if [ "$POD_ROLE" == "MANAGER" ]; then
    echo "Running Manager Role"
    /opt/enable-vcp-scripts/manager_pod.sh
elif [ "$POD_ROLE" == "DAEMON" ]; then
    ls /host/tmp/vcp-configuration-complete &> /dev/null
    if [ $? -eq 0 ]; then
        if [ "$k8s_secret_roll_back_switch" == "on" ]; then
            perform_rollback "$k8s_secret_config_backup" "$k8s_secret_kubernetes_api_server_manifest" "$k8s_secret_kubernetes_controller_manager_manifest" "$k8s_secret_kubernetes_kubelet_service_configuration_file"
        fi
        # Daemon Pod has already completed VCP Configuration, So Pause infinity
        echo "[INFO] Done with all tasks. Sleeping Infinity."
        python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
    fi
    echo "Running Daemon Role"
    bash /opt/enable-vcp-scripts/daemonset_pod.sh
else
    echo "[ERROR] Invalid Role"; 
    exit $ERROR_INVALID_POD_ROLE;
fi

echo "[INFO] Done with all tasks. Sleeping Infinity."
# sleep infinity
python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
