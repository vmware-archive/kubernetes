#!/bin/bash
source $(dirname "$0")/exit_codes.sh

DAEMONSET_SCRIPT_PHASE1="[PHASE 1] Validation"
DAEMONSET_SCRIPT_PHASE2="[PHASE 2] Enable disk.enableUUID on the VM"
DAEMONSET_SCRIPT_PHASE3="[PHASE 3] Move VM to the Working Directory"
DAEMONSET_SCRIPT_PHASE4="[PHASE 4] Validate and backup existing node configuration"
DAEMONSET_SCRIPT_PHASE5="[PHASE 5] Create vSphere.conf file"
DAEMONSET_SCRIPT_PHASE6="[PHASE 6] Update Manifest files and service configuration file"
DAEMONSET_SCRIPT_PHASE7="[PHASE 7] Restart Kubelet Service"

DAEMONSET_PHASE_RUNNING="RUNNING"
DAEMONSET_PHASE_FAILED="FAILED"
DAEMONSET_PHASE_COMPLETE="COMPLETE"

read_secret_keys() {
    export k8s_secret_master_node_name=`cat /secret-volume/master_node_name; echo;`
    export k8s_secret_vc_admin_username=`cat /secret-volume/vc_admin_username; echo;`
    export k8s_secret_vc_admin_password=`cat /secret-volume/vc_admin_password; echo;`
    export k8s_secret_vcp_username=`cat /secret-volume/vcp_username; echo;`
    export k8s_secret_vcp_password=`cat /secret-volume/vcp_password; echo;`
    export k8s_secret_vc_ip=`cat /secret-volume/vc_ip; echo;`
    export k8s_secret_vc_port=`cat /secret-volume/vc_port; echo;`
    export k8s_secret_datacenter=`cat /secret-volume/datacenter; echo;`
    export k8s_secret_default_datastore=`cat /secret-volume/default_datastore; echo;`
    export k8s_secret_node_vms_folder=`cat /secret-volume/node_vms_folder; echo;`
    export k8s_secret_node_vms_cluster_or_host=`cat /secret-volume/node_vms_cluster_or_host; echo;`
    export k8s_secret_vcp_configuration_file_location=`cat /secret-volume/vcp_configuration_file_location; echo;`
    export k8s_secret_kubernetes_api_server_manifest=`cat /secret-volume/kubernetes_api_server_manifest; echo;`
    export k8s_secret_kubernetes_controller_manager_manifest=`cat /secret-volume/kubernetes_controller_manager_manifest; echo;`
    export k8s_secret_kubernetes_kubelet_service_name=`cat /secret-volume/kubernetes_kubelet_service_name; echo;`
    export k8s_secret_kubernetes_kubelet_service_configuration_file=`cat /secret-volume/kubernetes_kubelet_service_configuration_file; echo;`
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
            if [ $? -eq 1 ]; then
              echo "[ERROR] Failed to list the Role:" $ROLE_NAME
              exit $ERROR_VC_OBJECT_NOT_FOUND;
            fi
        else
            echo "[ERROR] Failed to create the Role:" $ROLE_NAME
            exit $ERROR_ROLE_CREATE;
        fi
    fi
}

wait_for_role_to_exist() {
    ROLE_NAME=$1
    for i in `seq 1 60`
    do
        govc role.ls $ROLE_NAME &> /dev/null
        if [ $? -eq 0 ]; then
            return 0
        fi
        sleep 1
    done
    return 1
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
        echo "[INFO] Role:["$ROLE_NAME"] assigned to the User:['$vcp_user'] on Entity:['$ENTITY']"
    else
        echo "[ERROR] Failed to Assign Role:["$ROLE_NAME"] to the User:['$vcp_user'] on Entity:['$ENTITY']"
        exit $ERROR_ASSIGN_ROLE;
    fi
}

locate_validate_and_backup_files() {
    PHASE=$DAEMONSET_SCRIPT_PHASE4
    CONFIG_FILE=$1
    BACKUP_DIR=$2
    POD_NAME=$3

    ls $CONFIG_FILE &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Found file:" $CONFIG_FILE
        if [ "${CONFIG_FILE##*.}" == "json" ]; then
            jq "." $CONFIG_FILE &> /dev/null
            if [ $? -eq 0 ]; then
                echo "[INFO] Verified " $CONFIG_FILE " is a Valid JSON file"
            else
                ERROR_MSG="Failed to Validate JSON for file:" $CONFIG_FILE
                update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
                exit $ERROR_FAIL_TO_PARSE_CONFIG_FILE
            fi
        elif [ "${CONFIG_FILE##*.}" == "yaml" ]; then
            j2y -r $CONFIG_FILE &> /dev/null
            if [ $? -eq 0 ]; then
                echo "[INFO] Verified " $CONFIG_FILE " is a Valid YAML file"
            else
                ERROR_MSG="Failed to Validate YAML for file:" $CONFIG_FILE
                update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
                exit $ERROR_FAIL_TO_PARSE_CONFIG_FILE
            fi
        fi
        cp $CONFIG_FILE $BACKUP_DIR
        if [ $? -eq 0 ]; then
            echo "[INFO] Successfully backed up " $CONFIG_FILE at $BACKUP_DIR
        else
            ERROR_MSG="Failed to back up " $CONFIG_FILE at $BACKUP_DIR
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_BACKUP_FILE
        fi
    fi
}

add_flags_to_manifest_file() {
    PHASE=$DAEMONSET_SCRIPT_PHASE6
    MANIFEST_FILE=$1
    POD_NAME=$2
    
    commandflag=`jq '.spec.containers[0].command' ${MANIFEST_FILE} | grep "\-\-cloud-provider=vsphere"`
    if [ -z "$commandflag" ]; then
        # adding --cloud-provider=vsphere flag to the manifest file
        jq '.spec.containers[0].command |= .+ ["--cloud-provider=vsphere"]' ${MANIFEST_FILE} > ${MANIFEST_FILE}
        if [ $? -eq 0 ]; then
            echo "[INFO] Sucessfully added --cloud-provider=vsphere flag to ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add --cloud-provider=vsphere flag to ${MANIFEST_FILE}"
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-provider=vsphere flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi

    commandflag=`jq '.spec.containers[0].command' ${MANIFEST_FILE} | grep "\-\-cloud-config=${k8s_secret_vcp_configuration_file_location}/vsphere.conf"`
    if [ -z "$commandflag" ]; then
        # adding --cloud-config=/k8s_secret_vcp_configuration_file_location/vsphere.conf flag to the manifest file
        jq '.spec.containers[0].command |= .+ ["--cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf"]' ${MANIFEST_FILE} > ${MANIFEST_FILE}
        if [ $? -eq 0 ]; then
            echo "[INFO] Sucessfully added --cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag to ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add --cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag to ${MANIFEST_FILE}"
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi
}

init_VcpConigStatus() {
    POD_NAME="$1"
    INIT_STATUS="TPR Object for Pods Status is Created."
    cat <<EOF | kubectl create --save-config -f -
    apiVersion: "vmware.com/v1"
    kind: VcpStatus
    metadata:
        name: $POD_NAME
    spec:
        phase: "$PHASE"
        status: "$INIT_STATUS"
        error: ""
EOF
}

update_VcpConigStatus() {
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
kubectl apply -f /tmp/${POD_NAME}_daemonset_status_update.yaml
}
