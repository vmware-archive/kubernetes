#!/bin/bash
# shellcheck source=./exit_codes.sh
source "$(dirname "$0")"/exit_codes.sh

export DAEMONSET_SCRIPT_PHASE1="[PHASE 1] Validation"
export DAEMONSET_SCRIPT_PHASE2="[PHASE 2] Enable disk.enableUUID on the VM"
export DAEMONSET_SCRIPT_PHASE3="[PHASE 3] Move VM to the Working Directory"
export DAEMONSET_SCRIPT_PHASE4="[PHASE 4] Validate and backup existing node configuration"
export DAEMONSET_SCRIPT_PHASE5="[PHASE 5] Create vSphere.conf file"
export DAEMONSET_SCRIPT_PHASE6="[PHASE 6] Update Manifest files and service configuration file"
export DAEMONSET_SCRIPT_PHASE7="[PHASE 7] Restart Kubelet Service"
export DAEMONSET_SCRIPT_PHASE8="COMPLETE"
export DAEMONSET_PHASE_RUNNING="RUNNING"
export DAEMONSET_PHASE_FAILED="FAILED"
export DAEMONSET_PHASE_COMPLETE="COMPLETE"

export K8S_SECRET_ROLL_BACK_SWITCH
export K8S_SECRET_CONFIG_BACKUP
export K8S_SECRET_VC_ADMIN_USERNAME
export K8S_SECRET_VC_ADMIN_PASSWORD
export K8S_SECRET_VCP_USERNAME
export K8S_SECRET_VCP_PASSWORD
export K8S_SECRET_VC_IP
export K8S_SECRET_VC_PORT
export K8S_SECRET_DATACENTER
export K8S_SECRET_DEFAULT_DATASTORE
export K8S_SECRET_NODE_VMS_FOLDER
export K8S_SECRET_NODE_VMS_CLUSTER_OR_HOST
export K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION
export K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST
export K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST
export K8S_SECRET_KUBERNETES_KUBELET_SERVICE_NAME
export K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE

read_secret_keys() {
    K8S_SECRET_ROLL_BACK_SWITCH=`cat /secret-volume/enable_roll_back_switch; echo;`
    K8S_SECRET_CONFIG_BACKUP=`cat /secret-volume/configuration_backup_directory; echo;`
    K8S_SECRET_VC_ADMIN_USERNAME=`cat /secret-volume/vc_admin_username; echo;`
    K8S_SECRET_VC_ADMIN_PASSWORD=`cat /secret-volume/vc_admin_password; echo;`
    K8S_SECRET_VCP_USERNAME=`cat /secret-volume/vcp_username; echo;`
    K8S_SECRET_VCP_PASSWORD=`cat /secret-volume/vcp_password; echo;`
    K8S_SECRET_VC_IP=`cat /secret-volume/vc_ip; echo;`
    K8S_SECRET_VC_PORT=`cat /secret-volume/vc_port; echo;`
    K8S_SECRET_DATACENTER=`cat /secret-volume/datacenter; echo;`
    K8S_SECRET_DEFAULT_DATASTORE=`cat /secret-volume/default_datastore; echo;`
    K8S_SECRET_NODE_VMS_FOLDER=`cat /secret-volume/node_vms_folder; echo;`
    K8S_SECRET_NODE_VMS_CLUSTER_OR_HOST=`cat /secret-volume/node_vms_cluster_or_host; echo;`
    K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION=`cat /secret-volume/vcp_configuration_file_location; echo;`
    K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST=`cat /secret-volume/kubernetes_api_server_manifest; echo;`
    K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST=`cat /secret-volume/kubernetes_controller_manager_manifest; echo;`
    K8S_SECRET_KUBERNETES_KUBELET_SERVICE_NAME=`cat /secret-volume/kubernetes_kubelet_service_name; echo;`
    K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE=`cat /secret-volume/kubernetes_kubelet_service_configuration_file; echo;`

    [ -z "$K8S_SECRET_ROLL_BACK_SWITCH" ] && { echo "[ERROR] K8S_SECRET_ROLL_BACK_SWITCH is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_CONFIG_BACKUP" ] && { echo "[ERROR] K8S_SECRET_CONFIG_BACKUP is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VC_ADMIN_USERNAME" ] && { echo "[ERROR] K8S_SECRET_VC_ADMIN_USERNAME is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VC_ADMIN_PASSWORD" ] && { echo "[ERROR] K8S_SECRET_VC_ADMIN_PASSWORD is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VCP_USERNAME" ] && { echo "[ERROR] K8S_SECRET_VCP_USERNAME is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VCP_PASSWORD" ] && { echo "[ERROR] K8S_SECRET_VCP_PASSWORD is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VC_IP" ] && { echo "[ERROR] K8S_SECRET_VC_IP is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VC_PORT" ] && { echo "[ERROR] K8S_SECRET_VC_PORT is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_DATACENTER" ] && { echo "[ERROR] K8S_SECRET_DATACENTER is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_DEFAULT_DATASTORE" ] && { echo "[ERROR] K8S_SECRET_DEFAULT_DATASTORE is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_NODE_VMS_FOLDER" ] && { echo "[ERROR] K8S_SECRET_NODE_VMS_FOLDER is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_NODE_VMS_CLUSTER_OR_HOST" ] && { echo "[ERROR] K8S_SECRET_NODE_VMS_CLUSTER_OR_HOST is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION" ] && { echo "[ERROR] K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST" ] && { echo "[ERROR] K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST" ] && { echo "[ERROR] K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_NAME" ] && { echo "[ERROR] K8S_SECRET_KUBERNETES_KUBELET_SERVICE_NAME is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
    [ -z "$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE" ] && { echo "[ERROR] K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE is not set"; exit $ERROR_SECRET_FILE_VALIDATION_FAILED; }
}

create_script_for_restarting_kubelet() {
echo '#!/bin/sh
systemctl daemon-reload
systemctl restart ${K8S_SECRET_KUBERNETES_KUBELET_SERVICE_NAME}
' > /host/tmp/restart_kubelet.sh
    chmod +x /host/tmp/restart_kubelet.sh
}

create_role() {
    ROLE_NAME=$1
    govc role.ls $ROLE_NAME &> /dev/null
    if [ $? -eq 1 ]; then
        echo "[INFO] Creating Role:" $ROLE_NAME
        govc role.create $ROLE_NAME &> /dev/null
        if [ $? -eq 0 ]; then
            echo "[INFO] Role:" $ROLE_NAME " created successfully"
            wait_for_role_to_exist $ROLE_NAME
        else
            echo "[ERROR] Failed to create the Role:" $ROLE_NAME
            exit $ERROR_ROLE_CREATE;
        fi
    fi
}

wait_for_role_to_exist() {
    ROLE_NAME=$1
    retry_attempt=1
    until govc role.ls $ROLE_NAME &> /dev/null; do
        ((retry_attempt++))
        if [ $retry_attempt -eq 12 ]; then
            echo "[ERROR] Failed to find the Role: ${ROLE_NAME}. Completed 12 attempt."
            exit $ERROR_VC_OBJECT_NOT_FOUND
        fi
        sleep 5
    done
    echo "[INFO] Verified ROLE: ${ROLE_NAME} is available in the vCenter Inventory"
}

assign_previledges_to_role() {
    ROLE_NAME=$1
    PREVILEDGES=$2
    update_role_command="govc role.update $ROLE_NAME $PREVILEDGES &> /dev/null"
    echo "[INFO] Adding Previledges to the Role:" $ROLE_NAME
    eval "$update_role_command"
    if [ $? -eq 0 ]; then
        echo "[INFO] Previledges added to the Role:" $ROLE_NAME
    else
        echo "[ERROR] Failed to add Previledges:['$PREVILEDGES'] to the Role:" $ROLE_NAME
        exit $ERROR_ADD_PRIVILEGES;
    fi
}

assign_role_to_user_and_entity() {
    vcp_user=$1
    ROLE_NAME=$2
    ENTITY=$3
    PROPAGATE=$4
    govc permissions.set -principal $vcp_user -propagate=$PROPAGATE -role $ROLE_NAME "$ENTITY" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Role:['$ROLE_NAME'] assigned to the User:['$vcp_user'] on Entity:['$ENTITY']"
    else
        echo "[ERROR] Failed to Assign Role:['$ROLE_NAME'] to the User:['$vcp_user'] on Entity:['$ENTITY']"
        exit $ERROR_ASSIGN_ROLE;
    fi
}

locate_validate_and_backup_files() {
    PHASE=$DAEMONSET_SCRIPT_PHASE4
    CONFIG_FILE=$1
    BACKUP_DIR=$2
    POD_NAME=$3

    file_name="${CONFIG_FILE##*/}"
    ls $BACKUP_DIR/$file_name &> /dev/null
    if [ $? -ne 0 ]; then
            ls $CONFIG_FILE &> /dev/null
            if [ $? -eq 0 ]; then
                echo "[INFO] Found file:" $CONFIG_FILE
                if [ "${CONFIG_FILE##*.}" == "json" ]; then
                    jq "." $CONFIG_FILE &> /dev/null
                    if [ $? -eq 0 ]; then
                        echo "[INFO] Verified " $CONFIG_FILE " is a Valid JSON file"
                    else
                        ERROR_MSG="Failed to Validate JSON for file: ${CONFIG_FILE}"
                        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
                        exit $ERROR_FAIL_TO_PARSE_CONFIG_FILE
                    fi
                elif [ "${CONFIG_FILE##*.}" == "yaml" ]; then
                    yaml2json $CONFIG_FILE
                    if [ $? -eq 0 ]; then
                        echo "[INFO] Verified " $CONFIG_FILE " is a Valid YAML file"
                    else
                        ERROR_MSG="Failed to Validate YAML for file: ${CONFIG_FILE}"
                        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
                        exit $ERROR_FAIL_TO_PARSE_CONFIG_FILE
                    fi
                fi
                cp $CONFIG_FILE $BACKUP_DIR
                if [ $? -eq 0 ]; then
                    echo "[INFO] Successfully backed up " $CONFIG_FILE at $BACKUP_DIR
                else
                    ERROR_MSG="Failed to back up " $CONFIG_FILE at $BACKUP_DIR
                    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
                    exit $ERROR_FAIL_TO_BACKUP_FILE
                fi
            fi
    else
        echo "[INFO] Skipping Backup - File: ${file_name} already present at the back up directory: ${BACKUP_DIR}"
    fi
}

add_flags_to_manifest_file() {
    PHASE=$DAEMONSET_SCRIPT_PHASE6
    MANIFEST_FILE=$1
    POD_NAME=$2

    commandflag=`jq '.spec.containers[0].command' ${MANIFEST_FILE} | grep "\-\-cloud-provider=vsphere"`
    if [ -z "$commandflag" ]; then
        # adding --cloud-provider=vsphere flag to the manifest file
        jq '.spec.containers[0].command |= .+ ["--cloud-provider=vsphere"]' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Sucessfully added --cloud-provider=vsphere flag to ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add --cloud-provider=vsphere flag to ${MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-provider=vsphere flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi

    commandflag=`jq '.spec.containers[0].command' ${MANIFEST_FILE} | grep "\-\-cloud-config=${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}/vsphere.conf"`
    if [ -z "$commandflag" ]; then
        # adding --cloud-config=/K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf flag to the manifest file
        jq '.spec.containers[0].command |= .+ ["--cloud-config='${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'/vsphere.conf"]' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Sucessfully added --cloud-config='${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}/vsphere.conf' flag to ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add --cloud-config='${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'/vsphere.conf flag to ${MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-config='${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'/vsphere.conf flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi

    ## If VCP configuration file path is not mounted on the containers, mount the path, so that containers can read vsphere.conf file
    volumepath=`jq '.spec.volumes' ${MANIFEST_FILE} | grep ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}`
    if [ -z "$volumepath" ]; then
        jq '.spec.volumes [.spec.volumes| length] |= . + { "hostPath": { "path": "'${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'" }, "name": "vsphereconf" }' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Suceessfully added volume: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} in the manifest file: ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add volume: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} in the manifest file: ${MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        fi
    else
        echo "[INFO] volume: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} is already available in the manifest file: ${MANIFEST_FILE}"
    fi

    mountpath=`jq '.spec.containers[0].volumeMounts' ${MANIFEST_FILE} | grep ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}`
    if [ -z "$mountpath" ]; then
        jq '.spec.containers[0].volumeMounts[.spec.containers[0].volumeMounts| length] |= . + { "mountPath": "'${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'", "name": "vsphereconf", "readOnly": true }' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Suceessfully added mount path: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} in the manifest file: ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add mount path: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} in the manifest file: ${MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        fi
    else
        echo "[INFO] Path: ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} is already mounted in the manifest file: ${MANIFEST_FILE}"
    fi
}

init_VcpConigStatus() {
    POD_NAME="$1"
    INIT_STATUS="TPR Object for Pods Status is Created."
    INIT_PHASE="CREATE"
    ERROR=""

echo "apiVersion: \"vmware.com/v1\"
kind: VcpStatus
metadata:
    name: $POD_NAME
spec:
    phase: "\"${INIT_PHASE}\""
    status: "\"${INIT_STATUS}\""
    error: "\"${ERROR}\""" > /tmp/${POD_NAME}_daemonset_status_create.yaml

retry_attempt=1
until kubectl create --save-config -f /tmp/${POD_NAME}_daemonset_status_create.yaml &> /dev/null; do
    ((retry_attempt++))
    if [ $retry_attempt -eq 12 ]; then
        echo "[ERROR] Failed to Create TPR for POD Status Update. Completed 12 attempt."
        exit $ERROR_FAILED_TO_CREATE_TPR
    fi
    sleep 5
done
}

update_VcpConfigStatus() {
    POD_NAME="$1"
    PHASE="$2"
    STATUS="$3"
    ERROR="$4"

    if [ "$STATUS" == "FAILED" ]; then
        echo "[ERROR] ${ERROR}"
    fi

echo "apiVersion: \"vmware.com/v1\"
kind: VcpStatus
metadata:
    name: $POD_NAME
spec:
    phase: "\"${PHASE}\""
    status: "\"${STATUS}\""
    error: "\"${ERROR}\""" > /tmp/${POD_NAME}_daemonset_status_update.yaml

retry_attempt=1
until kubectl apply -f /tmp/${POD_NAME}_daemonset_status_update.yaml &> /dev/null; do
    ((retry_attempt++))
    if [ $retry_attempt -eq 12 ]; then
        echo "[ERROR] Failed to Create TPR for POD Status Update. Completed 12 attempt."
        exit $ERROR_FAILED_TO_CREATE_TPR
    fi
    sleep 5
done
}

init_VcpConfigSummaryStatus() {
    TOTAL_NUMBER_OF_NODES="$1"
echo "apiVersion: \"vmware.com/v1\"
kind: VcpSummary
metadata:
    name: vcpinstallstatus
spec:
    nodes_in_phase1: 0
    nodes_in_phase2: 0
    nodes_in_phase3: 0
    nodes_in_phase4: 0
    nodes_in_phase5: 0
    nodes_in_phase6: 0
    nodes_in_phase7: 0
    nodes_being_configured: 0
    nodes_failed_to_configure: 0
    nodes_sucessfully_configured: 0
    total_number_of_nodes: "\"${TOTAL_NUMBER_OF_NODES}\""" > /tmp/enablevcpstatussummary.yaml

retry_attempt=1
until kubectl create --save-config -f /tmp/enablevcpstatussummary.yaml &> /dev/null; do
    ((retry_attempt++))
    if [ $retry_attempt -eq 12 ]; then
        echo "[ERROR] init_VcpConfigSummaryStatus failed. Completed 12 attempt."
        exit $ERROR_FAILED_TO_CREATE_TPR
    fi
    sleep 5
done
}

update_VcpConfigSummaryStatus() {
    TOTAL_NUMBER_OF_NODES="$1"

    VcpStatus_OBJECTS=`kubectl get VcpStatus --namespace=vmware -o json | jq '.items'`
    TOTAL_IN_PHASE1=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 1" | wc -l`
    TOTAL_IN_PHASE2=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 2" | wc -l`
    TOTAL_IN_PHASE3=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 3" | wc -l`
    TOTAL_IN_PHASE4=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 4" | wc -l`
    TOTAL_IN_PHASE5=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 5" | wc -l`
    TOTAL_IN_PHASE6=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 6" | wc -l`
    TOTAL_IN_PHASE7=`echo $VcpStatus_OBJECTS | jq '.[] .spec.phase' | grep "PHASE 7" | wc -l`
    TOTAL_WITH_RUNNING_STATUS=`echo $VcpStatus_OBJECTS | jq '.[] .spec.status' | grep "${DAEMONSET_PHASE_RUNNING}" | wc -l`
    TOTAL_WITH_FAILED_STATUS=`echo $VcpStatus_OBJECTS | jq '.[] .spec.status' | grep "${DAEMONSET_PHASE_FAILED}" | wc -l`
    TOTAL_WITH_COMPLETE_STATUS=`echo $VcpStatus_OBJECTS | jq '.[] .spec.status' | grep "${DAEMONSET_PHASE_COMPLETE}" | wc -l`

echo "apiVersion: \"vmware.com/v1\"
kind: VcpSummary
metadata:
    name: vcpinstallstatus
spec:
    nodes_in_phase1 : "\"${TOTAL_IN_PHASE1}\""
    nodes_in_phase2 : "\"${TOTAL_IN_PHASE2}\""
    nodes_in_phase3 : "\"${TOTAL_IN_PHASE3}\""
    nodes_in_phase4 : "\"${TOTAL_IN_PHASE4}\""
    nodes_in_phase5 : "\"${TOTAL_IN_PHASE5}\""
    nodes_in_phase6 : "\"${TOTAL_IN_PHASE6}\""
    nodes_in_phase7 : "\"${TOTAL_IN_PHASE7}\""
    nodes_being_configured : "\"${TOTAL_WITH_RUNNING_STATUS}\""
    nodes_failed_to_configure : "\"${TOTAL_WITH_FAILED_STATUS}\""
    nodes_sucessfully_configured : "\"${TOTAL_WITH_COMPLETE_STATUS}\""
    total_number_of_nodes: "\"${TOTAL_NUMBER_OF_NODES}\""" > /tmp/enablevcpstatussummary.yaml

retry_attempt=1
until kubectl apply -f /tmp/enablevcpstatussummary.yaml &> /dev/null; do
    ((retry_attempt++))
    if [ $retry_attempt -eq 12 ]; then
        echo "[ERROR] update_VcpConfigSummaryStatus failed. Completed 12 attempt."
        exit $ERROR_FAILED_TO_UPDATE_TPR
    fi
    sleep 5
done
}

perform_rollback() {
    K8S_SECRET_CONFIG_BACKUP="$1"
    K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST="$2"
    K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST="$3"
    K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE="$4"

    echo "[INFO - ROLLBACK] Starting Rollback"
    backupdir=/host${K8S_SECRET_CONFIG_BACKUP}
    ls $backupdir &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO - ROLLBACK] Copying manifest and service configuration files to their original location"
        api_server_manifest_file_name="${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST##*/}"
        controller_manager_manifest_file_name="${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST##*/}"
        kubelet_service_configuration_file_name="${K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE##*/}"

        ls ${backupdir}/${api_server_manifest_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${api_server_manifest_file_name} /host/${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST}
            echo "[INFO - ROLLBACK] Roll backed API Server manifest file: ${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST}"
        fi

        ls ${backupdir}/${controller_manager_manifest_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${controller_manager_manifest_file_name} /host/${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST}
            echo "[INFO - ROLLBACK] Roll backed controller-manager manifest file: ${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST}"
        fi

        ls ${backupdir}/${kubelet_service_configuration_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${kubelet_service_configuration_file_name} /host/${K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE}
            echo "[INFO - ROLLBACK] Roll backed kubelet service configuration file: ${K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE}"
        fi

        echo "[INFO - ROLLBACK] backed up files are rolled back. Restarting Kubelet"

        ls /host/tmp/${K8S_SECRET_CONFIG_BACKUP} &> /dev/null
        if [ $? -eq 0 ]; then
            # rename old backup directory
            timestamp=$(date +%s)
            mv /host/tmp/${K8S_SECRET_CONFIG_BACKUP} /host/tmp/${K8S_SECRET_CONFIG_BACKUP}-${timestamp}
        fi
        mv $backupdir /host/tmp/${K8S_SECRET_CONFIG_BACKUP}
        create_script_for_restarting_kubelet
        echo "[INFO] Reloading systemd manager configuration and restarting kubelet service"
        chroot /host /tmp/restart_kubelet.sh
        if [ $? -eq 0 ]; then
            touch /host/tmp/vcp-rollback-complete
            rm -rf /host/tmp/vcp-configuration-complete
            echo "[INFO - ROLLBACK] kubelet service restarted sucessfully"
        else
            echo "[ERROR - ROLLBACK] failed to restart kubelet after roll back"
        fi
    fi
}
