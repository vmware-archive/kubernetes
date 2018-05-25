#!/bin/bash
# shellcheck source=./common_func.sh
source "$(dirname "$0")"/common_func.sh
# shellcheck source=./exit_codes.sh
source "$(dirname "$0")"/exit_codes.sh

POD_NAME="$1"
NODE_NAME="$2"

[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
init_VcpConigStatus "$POD_NAME"

PHASE=$DAEMONSET_SCRIPT_PHASE1
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

[ -z "$NODE_NAME" ] && { update_VcpConfigStatus "${POD_NAME}" "${PHASE}" "${DAEMONSET_PHASE_FAILED}" "NODE_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME
# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys
backupdir=/host/${K8S_SECRET_CONFIG_BACKUP}

if [ "$K8S_SECRET_ROLL_BACK_SWITCH" == "on" ]; then
      ls /host/tmp/vcp-rollback-complete &> /dev/null
      if [ $? -eq 0 ]; then
          echo "[INFO] Found flag file: '/host/tmp/vcp-rollback-complete' Observed that VCP Configuration Rollback is complete"
      else
        perform_rollback "$K8S_SECRET_CONFIG_BACKUP" "$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST" "$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST" "$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE"
        echo "[INFO] Rollback complete"
      fi
      python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
fi

ls /host/tmp/vcp-configuration-complete &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found flag file: '/host/tmp/vcp-configuration-complete'. Observed that VCP Configuration is complete"
    python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
fi

PHASE=$DAEMONSET_SCRIPT_PHASE2
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

# connect to vCenter using VC Admin username and password
export GOVC_INSECURE=1
export GOVC_USERNAME=$K8S_SECRET_VC_ADMIN_USERNAME
export GOVC_PASSWORD=$K8S_SECRET_VC_ADMIN_PASSWORD
export GOVC_URL='https://'$K8S_SECRET_VC_IP':'$K8S_SECRET_VC_PORT'/sdk'

# Get VM's UUID, Find VM Path using VM UUID and set disk.enableUUID to 1 on the VM
vmuuid=$(cat /host/sys/class/dmi/id/product_serial | sed -e 's/^VMware-//' -e 's/-/ /' | awk '{ print tolower($1$2$3$4 "-" $5$6 "-" $7$8 "-" $9$10 "-" $11$12$13$14$15$16) }')
[ -z "$vmuuid" ] && { ERROR_MSG="Unable to get VM UUID from /host/sys/class/dmi/id/product_serial"; update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"; exit $ERROR_UNKNOWN; }

vmpath=$(govc vm.info -dc="${K8S_SECRET_DATACENTER}" -vm.uuid=$vmuuid | grep "Path:" | awk 'BEGIN {FS=":"};{print $2}' | tr -d ' ')
[ -z "$vmpath" ] && { ERROR_MSG="Unable to find VM using VM UUID: ${vmuuid}"; update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"; exit $ERROR_VC_OBJECT_NOT_FOUND; }

govc vm.change -e="disk.enableUUID=1" -vm="$vmpath" &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Successfully enabled disk.enableUUID flag on the Node Virtual Machine"
else
    ERROR_MSG="Failed to enable disk.enableUUID flag on the Node Virtual Machine"
    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
    exit $ERROR_ENABLE_DISK_UUID
fi

vmname=$(govc vm.info -dc="${K8S_SECRET_DATACENTER}" -vm.uuid=$vmuuid | grep "Name" | awk 'BEGIN {FS=":"};{print $2}' | tr -d ' ')
if [ "$vmname" != "$NODE_NAME" ]; then
    govc object.rename $vmpath $NODE_NAME
    if [ $? -eq 0 ]; then
    echo "[INFO] Successfully renamed node vm name from $vmname to $NODE_NAME"
    else
        ERROR_MSG="Failed to rename Node Virtual Machine from name: $vmname to $NODE_NAME"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_RENAME_NODE_VM
    fi
fi

PHASE=$DAEMONSET_SCRIPT_PHASE3
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

# Move Node VM to the VM Folder.
govc object.mv -dc=$K8S_SECRET_DATACENTER $vmpath $K8S_SECRET_NODE_VMS_FOLDER &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Moved Node Virtual Machine to the Working Directory Folder".
else
    ERROR_MSG="Failed to move Node Virtual Machine to the Working Directory Folder"
    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
    exit $ERROR_MOVE_NODE_TO_WORKING_DIR
fi

PHASE=$DAEMONSET_SCRIPT_PHASE4
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

# Creating back up directory for manifest files and kubelet service configuration file.
ls $backupdir &> /dev/null
if [ $? -ne 0 ]; then
    echo "[INFO] Creating directory: '${backupdir}' for back up of manifest files and kubelet service configuration file"
    mkdir -p $backupdir
    if [ $? -eq 0 ]; then
        echo "[INFO] Successfully created back up directory: ${backupdir} on ${NODE_NAME} node"
    else
        ERROR_MSG="Failed to create directory: '${backupdir}' for back up of manifest files and kubelet service configuration file"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAIL_TO_CREATE_BACKUP_DIRECTORY
    fi
fi

# Verify that the directory for the vSphere Cloud Provider configuration file is accessible.
ls /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified that the directory for the vSphere Cloud Provider configuration file is accessible. Path: /host/${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}"
else
    mkdir -p /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION
    if [ $? -ne 0 ]; then
        ERROR_MSG="Unable to Create Directory: /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION for vSphere Conf file"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_VSPHERE_CONF_DIRECTORY_NOT_PRESENT
    fi
    chmod 0750 /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION
    ls /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION &> /dev/null
    if [ $? -ne 0 ]; then
        ERROR_MSG="Directory (/host/${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}) for vSphere Cloud Provider Configuration file is not present"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_VSPHERE_CONF_DIRECTORY_NOT_PRESENT
    fi
fi

ls /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] vsphere.conf file is already available at /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf"
    cp /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf $backupdir/vsphere.conf
    if [ $? -eq 0 ]; then
        echo "[INFO] Existing vsphere.conf file is copied to ${backupdir}/vsphere.conf"
    else
        ERROR_MSG="Failed to back up vsphere.conf file at " $backupdir/vsphere.conf
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAIL_TO_BACKUP_FILE
    fi
fi

# locate and back up manifest files and kubelet service configuration file.
file=/host$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST
locate_validate_and_backup_files $file $backupdir $POD_NAME

file=/host$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST
locate_validate_and_backup_files $file $backupdir $POD_NAME

file=/host$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE
locate_validate_and_backup_files $file $backupdir $POD_NAME

PHASE=$DAEMONSET_SCRIPT_PHASE5
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

# Create vSphere Cloud Provider configuration file

ls /host/tmp/vsphere.conf &> /dev/null
if [ $? -ne 0 ]; then
    echo "[INFO] Creating vSphere Cloud Provider configuration file at /host/tmp/vsphere.conf"
    echo "[Global]
        user = ""\"${K8S_SECRET_VCP_USERNAME}"\""
        password = ""\"${K8S_SECRET_VCP_PASSWORD}"\""
        server = ""\"${K8S_SECRET_VC_IP}"\""
        port = ""\"${K8S_SECRET_VC_PORT}"\""
        insecure-flag = ""\"1"\""
        datacenter = ""\"${K8S_SECRET_DATACENTER}"\""
        datastore = ""\"${K8S_SECRET_DEFAULT_DATASTORE}"\""
        working-dir = ""\"${K8S_SECRET_NODE_VMS_FOLDER}"\""
    [Disk]
        scsicontrollertype = pvscsi" > /host/tmp/vsphere.conf

    if [ $? -eq 0 ]; then
        echo "[INFO] successfully created vSphere.conf file at : /host/tmp/vsphere.conf"
    else
        ERROR_MSG="Failed to create vsphere.conf file at : /host/tmp/vsphere.conf"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "FAILED" "$ERROR_MSG"
        exit $ERROR_FAIL_TO_CREATE_FILE
    fi
fi

PHASE=$DAEMONSET_SCRIPT_PHASE6
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

# update manifest files
ls /host/$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file: /host/$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST"
    if [ "${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST##*.}" == "json" ]; then
        MANIFEST_FILE="/host/tmp/kube-apiserver.json"
        cp /host/${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST} ${MANIFEST_FILE}
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed execute command: cp /host/${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST} ${MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAILED_TO_COPY_FILE
        fi
        add_flags_to_manifest_file $MANIFEST_FILE $POD_NAME
    elif [ "${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST##*.}" == "yaml" ]; then
        YAML_MANIFEST_FILE="/host/tmp/kube-apiserver.yaml"
        JSON_MANIFEST_FILE="/host/tmp/kube-apiserver.json"
        cp /host/${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST} ${YAML_MANIFEST_FILE}
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed execute command: cp /host/${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST} ${YAML_MANIFEST_FILE}"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAILED_TO_COPY_FILE
        fi
        # Convert YAML to JSON format
        yaml2json $YAML_MANIFEST_FILE > $JSON_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed to convert file from YAML to JSON format"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_J2Y_FAILURE
        fi
        add_flags_to_manifest_file $JSON_MANIFEST_FILE $POD_NAME
        # Convert JSON to YAML foramt
        json2yaml $JSON_MANIFEST_FILE > $YAML_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed to convert file from JSON to YAML format"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_J2Y_FAILURE
        fi
        rm -rf $JSON_MANIFEST_FILE
    else
        ERROR_MSG="Unsupported file format"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_UNSUPPORTED_FILE_FORMAT
    fi
fi

ls /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file: /host/${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST}"
    if [ "${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST##*.}" == "json" ]; then
        MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
        cp /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST $MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed execute command: cp /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST $MANIFEST_FILE"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAILED_TO_COPY_FILE
        fi
        add_flags_to_manifest_file $MANIFEST_FILE $POD_NAME
    elif [ "${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST##*.}" == "yaml" ]; then
        YAML_MANIFEST_FILE="/host/tmp/kube-controller-manager.yaml"
        JSON_MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
        cp /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST $YAML_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed execute command: /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST $YAML_MANIFEST_FILE"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_FAILED_TO_COPY_FILE
        fi
        # Convert YAML to JSON format
        yaml2json $YAML_MANIFEST_FILE > $JSON_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed to convert file from YAML to JSON format"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_J2Y_FAILURE
        fi
        add_flags_to_manifest_file $JSON_MANIFEST_FILE $POD_NAME
        # Convert JSON to YAML foramt
        json2yaml $JSON_MANIFEST_FILE > $YAML_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            ERROR_MSG="Failed to convert file from JSON to YAML format"
            update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
            exit $ERROR_J2Y_FAILURE
        fi
        rm -rf $JSON_MANIFEST_FILE
    else
        ERROR_MSG="Unsupported file format"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_UNSUPPORTED_FILE_FORMAT
    fi
fi

ls /host/$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file: /host/${K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE}"
    cp /host/$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE /host/tmp/kubelet-service-configuration
    if [ $? -ne 0 ]; then
        ERROR_MSG="Failed execute command: cp /host/$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE /host/tmp/kubelet-service-configuration"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAILED_TO_COPY_FILE
    fi
    eval "$(crudini --get --format=sh /host/tmp/kubelet-service-configuration Service ExecStart)"
    ExecStart=$(echo "${ExecStart//\\}")
    echo $ExecStart | grep "\-\-cloud-provider=vsphere" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] cloud-provider=vsphere flag is already present in the kubelet service configuration"
    else
        ExecStart=$(echo $ExecStart "--cloud-provider=vsphere")
    fi

    echo $ExecStart | grep "\-\-cloud-config=${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}/vsphere.conf" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] cloud-config='${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}'/vsphere.conf flag is already present in the kubelet service configuration"
    else
        ExecStart=$(echo $ExecStart "--cloud-config=${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}/vsphere.conf")
    fi

    echo $ExecStart | grep "docker run"  &> /dev/null
    if [ $? -eq 0 ]; then
        # If Kubelet is running in Docker Container, Need to mount directory where vsphere.conf file is located.
        # else skip this step
        echo $ExecStart | grep "${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}:${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}" &> /dev/null
        if [ $? -eq 0 ]; then
            echo "[INFO] Volume ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION} is already present in the kubelet service configuration"
        else
            addvolumeoption="docker run -v ${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}:${K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION}"
            ExecStart="${ExecStart/docker run/$addvolumeoption}"
        fi
    fi

    echo ExecStart="$ExecStart" | crudini --merge /host/tmp/kubelet-service-configuration Service
    if [ $? -eq 0 ]; then
        echo "[INFO] Sucessfully updated kubelet.service configuration"
    else
        ERROR_MSG="Failed to update kubelet.service configuration"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
    fi
fi

# Copying Updated files from /tmp to its Originial place.
IS_CONFIGURATION_UPDATED=false
UPDATED_MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
if [ "${K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST##*.}" == "yaml" ]; then
    UPDATED_MANIFEST_FILE="/host/tmp/kube-controller-manager.yaml"
fi
if [ -f $UPDATED_MANIFEST_FILE ]; then
    cp $UPDATED_MANIFEST_FILE /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST
    if [ $? -ne 0 ]; then
        ERROR_MSG="Failed execute command: cp $UPDATED_MANIFEST_FILE /host/$K8S_SECRET_KUBERNETES_CONTROLLER_MANAGER_MANIFEST"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAILED_TO_COPY_FILE
    fi
    IS_CONFIGURATION_UPDATED=true
fi

UPDATED_MANIFEST_FILE="/host/tmp/kube-apiserver.json"
if [ "${K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST##*.}" == "yaml" ]; then
    UPDATED_MANIFEST_FILE="/host/tmp/kube-apiserver.yaml"
fi
if [ -f $UPDATED_MANIFEST_FILE ]; then
    cp $UPDATED_MANIFEST_FILE /host/$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST
    if [ $? -ne 0 ]; then
        ERROR_MSG="Failed execute command: cp $UPDATED_MANIFEST_FILE /host/$K8S_SECRET_KUBERNETES_API_SERVER_MANIFEST"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAILED_TO_COPY_FILE
    fi
    IS_CONFIGURATION_UPDATED=true
fi

if [ -f /host/tmp/vsphere.conf ]; then
    cp /host/tmp/vsphere.conf /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf
    if [ $? -ne 0 ]; then
        ERROR_MSG="Failed execute command: cp /host/tmp/vsphere.conf /host/$K8S_SECRET_VCP_CONFIGURATION_FILE_LOCATION/vsphere.conf"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAILED_TO_COPY_FILE
    fi
fi

if [ -f /host/tmp/kubelet-service-configuration ]; then
    cp /host/tmp/kubelet-service-configuration /host/$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE
    if [ $? -ne 0 ]; then
        ERROR_MSG="Failed execute command: cp /host/tmp/kubelet-service-configuration /host/$K8S_SECRET_KUBERNETES_KUBELET_SERVICE_CONFIGURATION_FILE"
        update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
        exit $ERROR_FAILED_TO_COPY_FILE
    fi
    IS_CONFIGURATION_UPDATED=true
fi

if [ "$IS_CONFIGURATION_UPDATED" == "false" ] ; then
    ERROR_MSG="No configuration change is observed"
    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
    exit $ERROR_NO_CONFIGURATION_CHANGE_IS_OBSERVED
fi
PHASE=$DAEMONSET_SCRIPT_PHASE7
update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_RUNNING" ""

create_script_for_restarting_kubelet
echo "[INFO] Reloading systemd manager configuration and restarting kubelet service"
chroot /host /tmp/restart_kubelet.sh
if [ $? -eq 0 ]; then
    echo "[INFO] kubelet service restarted sucessfully"
    PHASE=$DAEMONSET_SCRIPT_PHASE8
    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_COMPLETE" ""
else
    ERROR_MSG="Failed to restart kubelet service"
    update_VcpConfigStatus "$POD_NAME" "$PHASE" "$DAEMONSET_PHASE_FAILED" "$ERROR_MSG"
fi
rm -rf /host/tmp/vcp-rollback-complete
touch /host/tmp/vcp-configuration-complete
