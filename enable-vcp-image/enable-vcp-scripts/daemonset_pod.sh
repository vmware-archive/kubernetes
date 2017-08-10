#!/bin/bash
source $(dirname "$0")/common_func.sh
source $(dirname "$0")/exit_codes.sh

[ -z "$POD_NAME" ] && { echo "[ERROR] POD_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$NODE_NAME" ] && { echo "[ERROR] NODE_NAME is not set"; exit $ERROR_POD_ENV_VALIDATION; }
[ -z "$POD_ROLE" ] && { echo "[ERROR] POD_ROLE is not set"; exit $ERROR_POD_ENV_VALIDATION; }

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME
# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

if [ "$k8s_secret_master_node_name" == "$NODE_NAME" ]; then
    echo "Daemonset pod is running on the master node"
else
    echo "Daemonset pod is running on the worker node"
fi

# connect to vCenter using VC Admin username and password
export GOVC_INSECURE=1
export GOVC_URL='https://'$k8s_secret_vc_admin_username':'$k8s_secret_vc_admin_password'@'$k8s_secret_vc_ip':'$k8s_secret_vc_port'/sdk'

# Get VM's UUID, Find VM Path using VM UUID and set disk.enableUUID to 1 on the VM
vmuuid=$(cat /host/sys/class/dmi/id/product_serial | sed -e 's/^VMware-//' -e 's/-/ /' | awk '{ print tolower($1$2$3$4 "-" $5$6 "-" $7$8 "-" $9$10 "-" $11$12$13$14$15$16) }')
[ -z "$vmuuid" ] && { echo "[ERROR] Unable to get VM UUID from /host/sys/class/dmi/id/product_serial"; exit $ERROR_UNKNOWN; }

vmpath=$(govc vm.info -vm.uuid=$vmuuid | grep "Path:" | awk 'BEGIN {FS=":"};{print $2}' | tr -d ' ')
[ -z "$vmpath" ] && { echo "[ERROR] Unable to find VM using VM UUID:" $vmuuid ; exit $ERROR_VC_OBJECT_NOT_FOUND; }

govc vm.change -e="disk.enableUUID=1" -vm="$vmpath" &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Successfully enabled disk.enableUUID flag on the Node Virtual Machine".
else
    echo "[ERROR] Failed to enable disk.enableUUID flag on the Node Virtual Machine"
    exit $ERROR_ENABLE_DISK_UUID
fi

# Move Node VM to the VM Folder.
govc object.mv $vmpath $k8s_secret_node_vms_folder &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Moved Node Virtual Machine to the Working Directory Folder".
else
    echo "[ERROR] Failed to move Node Virtual Machine to the Working Directory Folder"
    exit $ERROR_MOVE_NODE_TO_WORKING_DIR
fi

# Creating back up directory for manifest files and kubelet service configuration file.
echo "[INFO] Creating directory: '/host/tmp/$POD_NAME' for back up of manifest files and kubelet service configuration file"
mkdir /host/tmp/$POD_NAME
if [ $? -eq 0 ]; then
    echo "[INFO] Successfully created back up directory:" /host/tmp/$POD_NAME " on $NODE_NAME node"
else
    echo "[ERROR] Failed to create directory: '/host/tmp/$POD_NAME' for back up of manifest files and kubelet service configuration file"
    exit $ERROR_FAIL_TO_CREATE_BACKUP_DIRECTORY
fi

# Verify that the directory for the vSphere Cloud Provider configuration file is accessible.
ls /host/$k8s_secret_vcp_configuration_file_location &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified that the directory for the vSphere Cloud Provider configuration file is accessible. Path: ("/host/$k8s_secret_vcp_configuration_file_location ")"
else
    echo "[ERROR] Directory (/host/$k8s_secret_vcp_configuration_file_location) for vSphere Cloud Provider Configuration file is not present"
    exit $ERROR_VSPHERE_CONF_DIRECTORY_NOT_PRESENT
fi

ls /host/$k8s_secret_vcp_configuration_file_location/vsphere.conf &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] vsphere.conf file is already available at /host/$k8s_secret_vcp_configuration_file_location/vsphere.conf"
    cp /host/$k8s_secret_vcp_configuration_file_location/vsphere.conf /host/tmp/$POD_NAME
    if [ $? -eq 0 ]; then
        echo "[INFO] Existing vsphere.conf file is copied to" /host/tmp/$POD_NAME/vsphere.conf
    else
        echo "[ERROR] Failed to back up vsphere.conf file at " /host/tmp/$POD_NAME/vsphere.conf
        exit $ERROR_FAIL_TO_BACKUP_FILE
    fi
fi

# locate and back up manifest files and kubelet service configuration file.
file=/host/$k8s_secret_kubernetes_api_server_manifest
backupdir=/host/tmp/$POD_NAME
locate_validate_and_backup_files $file $backupdir

file=/host/$k8s_secret_kubernetes_controller_manager_manifest
locate_validate_and_backup_files $file $backupdir

file=/host/$k8s_secret_kubernetes_kubelet_service_configuration_file
locate_validate_and_backup_files $file $backupdir

# Create vSphere Cloud Provider configuration file
echo "[INFO] Creating vSphere Cloud Provider configuration file at /host/tmp/vsphere.conf"
echo "[Global]
    user = ""\"${k8s_secret_vc_admin_username}"\""
    password = ""\"${k8s_secret_vc_admin_password}"\""
    server = ""\"${k8s_secret_vc_ip}"\""
    port = ""\"${k8s_secret_vc_port}"\""
    insecure-flag = ""\"1"\""
    datacenter = ""\"${k8s_secret_datacenter}"\""
    datastore = ""\"${k8s_secret_default_datastore}"\""
    working-dir = ""\"${k8s_secret_node_vms_folder}"\""
[Disk]
    scsicontrollertype = pvscsi" > /host/tmp/vsphere.conf

if [ $? -eq 0 ]; then
    echo "[INFO] successfully created vSphere.conf file at :" /host/tmp/vsphere.conf
else
    echo "[ERROR] Failed to create vsphere.conf file at :" /host/tmp/vsphere.conf
    exit $ERROR_FAIL_TO_CREATE_FILE
fi

# update manifest files
ls /host/$k8s_secret_kubernetes_api_server_manifest &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file:" /host/$k8s_secret_kubernetes_api_server_manifest
    if [ "${k8s_secret_kubernetes_api_server_manifest##*.}" == "json" ]; then
        MANIFEST_FILE="/host/tmp/kube-apiserver.json"
        cp /host/$k8s_secret_kubernetes_api_server_manifest $MANIFEST_FILE
        add_flags_to_manifest_file $MANIFEST_FILE
    elif [ "${k8s_secret_kubernetes_api_server_manifest##*.}" == "yaml" ]; then
        YAML_MANIFEST_FILE="/host/tmp/kube-apiserver.yaml"
        JSON_MANIFEST_FILE="/host/tmp/kube-apiserver.json"
        cp /host/$k8s_secret_kubernetes_api_server_manifest $YAML_MANIFEST_FILE
        # Convert YAML to JSON format
        j2y -r $YAML_MANIFEST_FILE > $JSON_MANIFEST_FILE
        add_flags_to_manifest_file $JSON_MANIFEST_FILE
        # Convert JSON to YAML foramt
        j2y $JSON_MANIFEST_FILE > $YAML_MANIFEST_FILE
        rm -rf $JSON_MANIFEST_FILE
    else
        echo "[ERROR] Unsupported file format"
        exit $ERROR_UNSUPPORTED_FILE_FORMAT
    fi
fi

ls /host/$k8s_secret_kubernetes_controller_manager_manifest &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file:" /host/$k8s_secret_kubernetes_controller_manager_manifest
    if [ "${k8s_secret_kubernetes_controller_manager_manifest##*.}" == "json" ]; then
        MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
        cp /host/$k8s_secret_kubernetes_controller_manager_manifest $MANIFEST_FILE
        add_flags_to_manifest_file $MANIFEST_FILE
    elif [ "${k8s_secret_kubernetes_controller_manager_manifest##*.}" == "yaml" ]; then
        YAML_MANIFEST_FILE="/host/tmp/kube-controller-manager.yaml"
        JSON_MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
        cp /host/$k8s_secret_kubernetes_controller_manager_manifest $YAML_MANIFEST_FILE
        # Convert YAML to JSON format
        j2y -r $YAML_MANIFEST_FILE > $JSON_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            echo "[ERROR] Failed to convert file from YAML to JSON format"
            exit $ERROR_J2Y_FAILURE
        fi
        add_flags_to_manifest_file $JSON_MANIFEST_FILE
        # Convert JSON to YAML foramt
        j2y $JSON_MANIFEST_FILE > $YAML_MANIFEST_FILE
        if [ $? -ne 0 ]; then
            echo "[ERROR] Failed to convert file from JSON to YAML format"
            exit $ERROR_J2Y_FAILURE
        fi
        rm -rf $JSON_MANIFEST_FILE
    else
        echo "[ERROR] Unsupported file format"
        exit $ERROR_UNSUPPORTED_FILE_FORMAT
    fi
fi

ls /host/$k8s_secret_kubernetes_kubelet_service_configuration_file &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Found file:" /host/$k8s_secret_kubernetes_kubelet_service_configuration_file
    cp /host/$k8s_secret_kubernetes_kubelet_service_configuration_file /host/tmp/kubelet-service-configuration
    eval $(crudini --get --format=sh /host/tmp/kubelet-service-configuration Service ExecStart)
    ExecStart=$(echo "${ExecStart//\\}")
    echo $ExecStart | grep "\-\-cloud-provider=vsphere" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] cloud-provider=vsphere flag is already present in the kubelet service configuration"
    else
        ExecStart=$(echo $ExecStart "--cloud-provider=vsphere")
    fi
    echo $ExecStart | grep "\-\-cloud-config=${k8s_secret_vcp_configuration_file_location}/vsphere.conf" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] cloud-config='${k8s_secret_vcp_configuration_file_location}'/vsphere.conf flag is already present in the kubelet service configuration"
    else
        ExecStart=$(echo $ExecStart "--cloud-config=${k8s_secret_vcp_configuration_file_location}/vsphere.conf")
    fi
    echo ExecStart="$ExecStart" | crudini --merge /host/tmp/kubelet-service-configuration Service
    if [ $? -eq 0 ]; then 
        echo "[INFO] Sucessfully updated kubelet.service configuration"
    else
        echo "[ERROR] Failed to update kubelet.service configuration"
        exit $ERROR_FAIL_TO_ADD_CONFIG_PARAMETER
    fi
fi

# Copying Updated files from /tmp to its Originial place.
UPDATED_MANIFEST_FILE="/host/tmp/kube-controller-manager.json"
if [ "${k8s_secret_kubernetes_controller_manager_manifest##*.}" == "yaml" ]; then
    UPDATED_MANIFEST_FILE="/host/tmp/kube-controller-manager.yaml"
fi
if [ -f $UPDATED_MANIFEST_FILE ]; then
    cp $UPDATED_MANIFEST_FILE /host/$k8s_secret_kubernetes_controller_manager_manifest
fi

UPDATED_MANIFEST_FILE="/host/tmp/kube-apiserver.json"
if [ "${k8s_secret_kubernetes_api_server_manifest##*.}" == "yaml" ]; then
    UPDATED_MANIFEST_FILE="/host/tmp/kube-apiserver.yaml"
fi
if [ -f $UPDATED_MANIFEST_FILE ]; then
    cp $$UPDATED_MANIFEST_FILE /host/$k8s_secret_kubernetes_api_server_manifest
fi

if [ -f /host/tmp/vsphere.conf ]; then
    cp /host/tmp/vsphere.conf /host/$k8s_secret_vcp_configuration_file_location/vsphere.conf
fi

if [ -f /host/tmp/kubelet-service-configuration ]; then
    cp /host/tmp/kubelet-service-configuration /host/$k8s_secret_kubernetes_kubelet_service_configuration_file
fi

echo "systemctl daemon-reload
systemctl restart ${k8s_secret_kubernetes_kubelet_service_name}
" > /host/tmp/restart_kubelet.sh
chmod +x /host/tmp/restart_kubelet.sh

echo "[INFO] Reloading systemd manager configuration and restarting kubelet service"
chroot /host /tmp/restart_kubelet.sh
if [ $? -eq 0 ]; then
    echo "[INFO] kubelet service restarted sucessfully"
else
    echo "[ERROR] Failed to restart kubelet service"
fi