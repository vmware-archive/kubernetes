#!/bin/bash
# shellcheck source=./common_func.sh
source "$(dirname "$0")"/common_func.sh
# shellcheck source=./exit_codes.sh
source "$(dirname "$0")"/exit_codes.sh

# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

create_daemonset () {
        version=`kubectl version --short --output json`
        minor=`echo $version | jq '.serverVersion.minor' | sed 's/"//g'`
        major=`echo $version | jq '.serverVersion.major' | sed 's/"//g'`

        resource_yaml=""
        if [ $minor -ge 8 ] && [ "$major" -ge 1 ]; then
                resource_yaml=/opt/enable-vcp-scripts/vcp-summary-crd.yaml
        else
                resource_yaml=/opt/enable-vcp-scripts/vcp-summary-tpr.yaml
        fi

        kubectl create -f "$resource_yaml"
        if [ $? -eq 0 ]; then
            echo "[INFO] Executed kubectl create command to create resources through $resource_yaml."
        else
            echo "[ERROR] 'kubectl create' failed to create resources through $resource_yaml."
        fi

        kubectl create -f /opt/enable-vcp-scripts/vcp-daemontset.yaml
        if [ $? -eq 0 ]; then
            echo "[INFO] Executed kubectl create command to create vcp-daemontset."
        else
            echo "[ERROR] 'kubectl create' failed to create vcp-daemonset."
        fi
}

if [ "$K8S_SECRET_ROLL_BACK_SWITCH" == "on" ]; then
  echo "[INFO] POD-MANAGER operations are skipped as K8S_SECRET_ROLL_BACK_SWITCH is set to on"
  create_daemonset
  python -c 'while 1: import ctypes; ctypes.CDLL(None).pause()'
fi

# connect to vCenter using VCP username and password
export GOVC_INSECURE=1
export GOVC_URL='https://'$K8S_SECRET_VCP_USERNAME':'$K8S_SECRET_VCP_PASSWORD'@'$K8S_SECRET_VC_IP':'$K8S_SECRET_VC_PORT'/sdk'
error_message=$(govc ls 2>&1 >/dev/null)

if [ $? -eq 1 ]; then
    if [[ $error_message == *"Cannot complete login due to an incorrect user name or password."* ]]; then
        echo "Failed to login to vCenter using VCP User:" $K8S_SECRET_VCP_USERNAME " and VCP Password specifed in the secret file"
        exit $ERROR_VC_LOGIN
    elif [[ $error_message == *"Permission to perform this operation was denied."* ]]; then
        echo "[INFO] Successfully able to login using VCP Username:" $K8S_SECRET_VCP_USERNAME
        echo "[INFO] Permissions will be added to User:" $K8S_SECRET_VCP_USERNAME "to allow performing Kubernetes Operations."
    else
        exit $ERROR_UNKNOWN
    fi
fi

# Capture Number of Registered Nodes including master before making any configuration change.
NUMBER_OF_REGISTERED_NODES=`kubectl get nodes -o json | jq '.items | length'`

 # connect to vCenter using VC Admin username and password
export GOVC_URL='https://'$K8S_SECRET_VC_ADMIN_USERNAME':'$K8S_SECRET_VC_ADMIN_PASSWORD'@'$K8S_SECRET_VC_IP':'$K8S_SECRET_VC_PORT'/sdk'
# Verify if the Datacenter exists or not.
govc datacenter.info $K8S_SECRET_DATACENTER &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Datacenter:" $K8S_SECRET_DATACENTER is present in the inventory.
else
    echo "[ERROR] Unable to find Datacenter:" $K8S_SECRET_DATACENTER.
    exit $ERROR_VC_OBJECT_NOT_FOUND;
fi

# Verify if the Datastore exists or not.
govc datastore.info -dc=$K8S_SECRET_DATACENTER $K8S_SECRET_DEFAULT_DATASTORE &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Datastore:" $K8S_SECRET_DEFAULT_DATASTORE is present in the inventory.
else
    echo "[ERROR] Unable to find Datastore:" $K8S_SECRET_DEFAULT_DATASTORE.
    exit $ERROR_VC_OBJECT_NOT_FOUND;
fi

# Check if the working directory VM folder exists. If not then create this folder
IFS="/"
vmFolders=($K8S_SECRET_NODE_VMS_FOLDER)
parentFolder=""
for vmFolder in "${vmFolders[@]}"
do
    govc folder.info -dc=$K8S_SECRET_DATACENTER "/$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Verified Node VMs Folder:" /$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder is present in the inventory.
    else
        echo "Creating folder: " /$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder
        govc folder.create "/$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder" &> /dev/null
        if [ $? -eq 0 ]; then
            echo "[INFO] Successfully created a new VM Folder:"/$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder
        else
            echo "[ERROR] Failed to create a vm folder:" /$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder
            exit $ERROR_FOLDER_CREATE;
        fi
    fi
    parentFolder=$parentFolder/$vmFolder
done

govc folder.info -dc=$K8S_SECRET_DATACENTER "/$K8S_SECRET_DATACENTER/vm/$K8S_SECRET_NODE_VMS_FOLDER" &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Node VMs Folder:" "/$K8S_SECRET_DATACENTER/vm/$K8S_SECRET_NODE_VMS_FOLDER" is present in the inventory.
else
    echo "[ERROR] Unable to find VM Folder:" "/$K8S_SECRET_DATACENTER/vm/$K8S_SECRET_NODE_VMS_FOLDER"
    exit $ERROR_VC_OBJECT_NOT_FOUND;
fi

# if Administrator user is passed as VCP user, then skip all Operations for VCP user.
if [ "$K8S_SECRET_VC_ADMIN_USERNAME" != "$K8S_SECRET_VCP_USERNAME" ]; then
    ROLE_NAME=manage-k8s-volumes
    create_role $ROLE_NAME
    PREVILEDGES="Datastore.AllocateSpace \
    Datastore.FileManagement \
    System.Anonymous \
    System.Read \
    System.View"
    assign_previledges_to_role $ROLE_NAME $PREVILEDGES

    ROLE_NAME=manage-k8s-node-vms
    create_role $ROLE_NAME
    PREVILEDGES="Resource.AssignVMToPool \
    System.Anonymous \
    System.Read \
    System.View \
    VirtualMachine.Config.AddExistingDisk \
    VirtualMachine.Config.AddNewDisk \
    VirtualMachine.Config.AddRemoveDevice \
    VirtualMachine.Config.RemoveDisk \
    VirtualMachine.Inventory.Create \
    VirtualMachine.Inventory.Delete"
    assign_previledges_to_role $ROLE_NAME $PREVILEDGES

    ROLE_NAME=k8s-system-read-and-spbm-profile-view
    create_role $ROLE_NAME
    PREVILEDGES="StorageProfile.View \
    System.Anonymous \
    System.Read \
    System.View"
    assign_previledges_to_role $ROLE_NAME $PREVILEDGES

    echo "[INFO] Assigining Role to the VCP user and entities"
    ROLE_NAME=k8s-system-read-and-spbm-profile-view
    PROPAGATE=false
    assign_role_to_user_and_entity $K8S_SECRET_VCP_USERNAME $ROLE_NAME "/" $PROPAGATE

    ROLE_NAME=ReadOnly
    ENTITY="$K8S_SECRET_DATACENTER"
    PROPAGATE=false
    assign_role_to_user_and_entity $K8S_SECRET_VCP_USERNAME $ROLE_NAME "$ENTITY" $PROPAGATE

    ROLE_NAME=manage-k8s-volumes
    ENTITY="/$K8S_SECRET_DATACENTER/datastore/$K8S_SECRET_DEFAULT_DATASTORE"
    PROPAGATE=false
    assign_role_to_user_and_entity $K8S_SECRET_VCP_USERNAME $ROLE_NAME "$ENTITY" $PROPAGATE

    IFS="/"
    vmFolders=($K8S_SECRET_NODE_VMS_FOLDER)
    parentFolder=""
    ROLE_NAME=manage-k8s-node-vms
    PROPAGATE=true
    for vmFolder in "${vmFolders[@]}"
    do
        ENTITY="/$K8S_SECRET_DATACENTER/vm/$parentFolder/$vmFolder"
        assign_role_to_user_and_entity $K8S_SECRET_VCP_USERNAME $ROLE_NAME "$ENTITY" $PROPAGATE
        parentFolder=$parentFolder/$vmFolder
    done

    ROLE_NAME=manage-k8s-node-vms
    ENTITY="/$K8S_SECRET_DATACENTER/host/$K8S_SECRET_NODE_VMS_CLUSTER_OR_HOST"
    PROPAGATE=true
    assign_role_to_user_and_entity $K8S_SECRET_VCP_USERNAME $ROLE_NAME "$ENTITY" $PROPAGATE
else
    echo "Skipping Operations for VCP user. VCP user and Administrator user is same."
fi

create_daemonset

init_VcpConfigSummaryStatus "$NUMBER_OF_REGISTERED_NODES"

while true
do
    error=$(kubectl get VcpStatus --namespace=vmware 2>&1 >/dev/null)
    if [ -z "$error" ]; then
        update_VcpConfigSummaryStatus "$NUMBER_OF_REGISTERED_NODES"
        TOTAL_WITH_COMPLETE_STATUS=`kubectl get VcpStatus --namespace=vmware -o json | jq '.items[] .spec.status' | grep "${DAEMONSET_PHASE_COMPLETE}" | wc -l`
        echo "[INFO] Waiting for [${NUMBER_OF_REGISTERED_NODES}] nodes to report successfully configured. Found [${TOTAL_WITH_COMPLETE_STATUS}] nodes configured successfully."
        if [ $TOTAL_WITH_COMPLETE_STATUS -eq $NUMBER_OF_REGISTERED_NODES ]; then
            # Need to update final status before exiting the loop, so that nodes_sucessfully_configured is reported correctly.
            update_VcpConfigSummaryStatus "$NUMBER_OF_REGISTERED_NODES"
            echo "[INFO] All Daemonset Pods has reached to the Complete Phase"
            break
        fi
    fi
    sleep 1
done
