#!/bin/bash
source $(dirname "$0")/exit_codes.sh

DAEMONSET_SCRIPT_PHASE1="[PHASE 1] Validation"
DAEMONSET_SCRIPT_PHASE2="[PHASE 2] Enable disk.enableUUID on the VM"
DAEMONSET_SCRIPT_PHASE3="[PHASE 3] Move VM to the Working Directory"
DAEMONSET_SCRIPT_PHASE4="[PHASE 4] Validate and backup existing node configuration"
DAEMONSET_SCRIPT_PHASE5="[PHASE 5] Create vSphere.conf file"
DAEMONSET_SCRIPT_PHASE6="[PHASE 6] Update Manifest files and service configuration file"
DAEMONSET_SCRIPT_PHASE7="[PHASE 7] Restart Kubelet Service"
DAEMONSET_SCRIPT_PHASE8="COMPLETE"
DAEMONSET_PHASE_RUNNING="RUNNING"
DAEMONSET_PHASE_FAILED="FAILED"
DAEMONSET_PHASE_COMPLETE="COMPLETE"

read_secret_keys() {
    export k8s_secret_roll_back_switch=`cat /secret-volume/enable_roll_back_switch; echo;`
    export k8s_secret_config_backup=`cat /secret-volume/configuration_backup_directory; echo;`
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

create_script_for_restarting_kubelet() {
echo '#!/bin/sh
systemctl daemon-reload
systemctl restart ${k8s_secret_kubernetes_kubelet_service_name}
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
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-provider=vsphere flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi

    commandflag=`jq '.spec.containers[0].command' ${MANIFEST_FILE} | grep "\-\-cloud-config=${k8s_secret_vcp_configuration_file_location}/vsphere.conf"`
    if [ -z "$commandflag" ]; then
        # adding --cloud-config=/k8s_secret_vcp_configuration_file_location/vsphere.conf flag to the manifest file
        jq '.spec.containers[0].command |= .+ ["--cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf"]' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Sucessfully added --cloud-config='${k8s_secret_vcp_configuration_file_location}/vsphere.conf' flag to ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add --cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag to ${MANIFEST_FILE}"
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
        fi
    else
        echo "[INFO] --cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag is already present in the manifest file: ${MANIFEST_FILE}"
    fi

    ## If VCP configuration file path is not mounted on the containers, mount the path, so that containers can read vsphere.conf file
    volumepath=`jq '.spec.volumes' ${MANIFEST_FILE} | grep ${k8s_secret_vcp_configuration_file_location}`
    if [ -z "$volumepath" ]; then
        jq '.spec.volumes [.spec.volumes| length] |= . + { "hostPath": { "path": "'${k8s_secret_vcp_configuration_file_location}'" }, "name": "vsphereconf" }' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Suceessfully added volume: ${k8s_secret_vcp_configuration_file_location} in the manifest file: ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add volume: ${k8s_secret_vcp_configuration_file_location} in the manifest file: ${MANIFEST_FILE}"
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        fi
    else
        echo "[INFO] volume: ${k8s_secret_vcp_configuration_file_location} is already available in the manifest file: ${MANIFEST_FILE}"
    fi

    mountpath=`jq '.spec.containers[0].volumeMounts' ${MANIFEST_FILE} | grep ${k8s_secret_vcp_configuration_file_location}`
    if [ -z "$mountpath" ]; then
        jq '.spec.containers[0].volumeMounts[.spec.containers[0].volumeMounts| length] |= . + { "mountPath": "'${k8s_secret_vcp_configuration_file_location}'", "name": "vsphereconf", "readOnly": true }' ${MANIFEST_FILE} > ${MANIFEST_FILE}.tmp
        if [ $? -eq 0 ]; then
            mv ${MANIFEST_FILE}.tmp ${MANIFEST_FILE}
            echo "[INFO] Suceessfully added mount path: ${k8s_secret_vcp_configuration_file_location} in the manifest file: ${MANIFEST_FILE}"
        else
            ERROR_MSG="Failed to add mount path: ${k8s_secret_vcp_configuration_file_location} in the manifest file: ${MANIFEST_FILE}"
            update_VcpConigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        fi
    else
        echo "[INFO] Path: ${k8s_secret_vcp_configuration_file_location} is already mounted in the manifest file: ${MANIFEST_FILE}"
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

    kubectl create --save-config -f /tmp/${POD_NAME}_daemonset_status_create.yaml
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

retry_attempt=1
until kubectl apply -f /tmp/${POD_NAME}_daemonset_status_update.yaml &> /dev/null || [ $retry_attempt -eq 12 ]; do
    sleep 5
    $(( retry_attempt++ ))
done
}

init_VcpConfigSummaryStatus() {
    TOTAL_NUMBER_OF_NODES="$1"
    cat <<EOF | kubectl create --save-config -f -
    apiVersion: "vmware.com/v1"
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
        total_number_of_nodes: "$TOTAL_NUMBER_OF_NODES"
EOF
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
until kubectl apply -f /tmp/enablevcpstatussummary.yaml &> /dev/null || [ $retry_attempt -eq 12 ]; do
    sleep 5
    $(( retry_attempt++ ))
done

}

perform_rollback() {
    k8s_secret_config_backup="$1"
    k8s_secret_kubernetes_api_server_manifest="$2"
    k8s_secret_kubernetes_controller_manager_manifest="$3"
    k8s_secret_kubernetes_kubelet_service_configuration_file="$4"

    echo "[INFO - ROLLBACK] Starting Rollback"
    backupdir=/host${k8s_secret_config_backup}
    ls $backupdir &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO - ROLLBACK] Copying manifest and service configuration files to their original location"
        api_server_manifest_file_name="${k8s_secret_kubernetes_api_server_manifest##*/}"
        controller_manager_manifest_file_name="${k8s_secret_kubernetes_controller_manager_manifest##*/}"
        kubelet_service_configuration_file_name="${k8s_secret_kubernetes_kubelet_service_configuration_file##*/}"

        ls ${backupdir}/${api_server_manifest_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${api_server_manifest_file_name} /host/${k8s_secret_kubernetes_api_server_manifest}
            echo "[INFO - ROLLBACK] Roll backed API Server manifest file: ${k8s_secret_kubernetes_api_server_manifest}"
        fi

        ls ${backupdir}/${controller_manager_manifest_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${controller_manager_manifest_file_name} /host/${k8s_secret_kubernetes_controller_manager_manifest}
            echo "[INFO - ROLLBACK] Roll backed controller-manager manifest file: ${k8s_secret_kubernetes_controller_manager_manifest}"
        fi

        ls ${backupdir}/${kubelet_service_configuration_file_name} &> /dev/null
        if [ $? -eq 0 ]; then
            cp ${backupdir}/${kubelet_service_configuration_file_name} /host/${k8s_secret_kubernetes_kubelet_service_configuration_file}
            echo "[INFO - ROLLBACK] Roll backed kubelet service configuration file: ${k8s_secret_kubernetes_kubelet_service_configuration_file}"
        fi

        echo "[INFO - ROLLBACK] backed up files are rolled back. Restarting Kubelet"
        
        ls /host/tmp/$backupdir &> /dev/null
        if [ $? -eq 0 ]; then
            # rename old backup directory
            timestamp=$(date +%s)
            mv /host/tmp/$backupdir /host/tmp/${backupdir}-${timestamp}
        fi
        mv $backupdir /host/tmp/
        create_script_for_restarting_kubelet
        echo "[INFO] Reloading systemd manager configuration and restarting kubelet service"
        chroot /host /tmp/restart_kubelet.sh
        if [ $? -eq 0 ]; then
            echo "[INFO - ROLLBACK] kubelet service restarted sucessfully"
        else
            echo "[ERROR - ROLLBACK] failed to restart kubelet after roll back"
        fi
    fi
}