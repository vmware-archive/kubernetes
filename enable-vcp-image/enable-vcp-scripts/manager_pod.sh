#!/bin/bash
source $(dirname "$0")/common_func.sh
source $(dirname "$0")/exit_codes.sh

# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

# connect to vCenter using VCP username and password
export GOVC_INSECURE=1
export GOVC_URL='https://'$k8s_secret_vcp_username':'$k8s_secret_vcp_password'@'$k8s_secret_vc_ip':'$k8s_secret_vc_port'/sdk'
error_message=$(govc ls 2>&1 >/dev/null)

if [ $? -eq 1 ]; then
    if [[ $error_message == *"Cannot complete login due to an incorrect user name or password."* ]]; then
        echo "Failed to login to vCenter using VCP User:" $k8s_secret_vcp_username " and VCP Password specifed in the secret file"
        exit $ERROR_VC_LOGIN
    elif [[ $error_message == *"Permission to perform this operation was denied."* ]]; then
        echo "[INFO] Successfully able to login using VCP Username:" $k8s_secret_vcp_username
        echo "[INFO] Permissions will be added to User:" $k8s_secret_vcp_username "to allow performing Kubernetes Operations."
    else
        exit $ERROR_UNKNOWN
    fi
fi

# Capture Number of Registered Nodes including master before making any configuration change.
NUMBER_OF_REGISTERED_NODES=`kubectl get nodes -o json | jq '.items | length'`

# if Administrator user is passed as VCP user, then skip all Operations for VCP user.
if [ "$k8s_secret_vc_admin_username" != "$k8s_secret_vcp_username" ]; then
    # connect to vCenter using VC Admin username and password
    export GOVC_URL='https://'$k8s_secret_vc_admin_username':'$k8s_secret_vc_admin_password'@'$k8s_secret_vc_ip':'$k8s_secret_vc_port'/sdk'
    # Verify if the Datacenter exists or not.
    govc datacenter.info $k8s_secret_datacenter &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Verified Datacenter:" $k8s_secret_datacenter is present in the inventory.
    else
        echo "[ERROR] Unable to find Datacenter:" $k8s_secret_datacenter.
        exit $ERROR_VC_OBJECT_NOT_FOUND;
    fi

    # Verify if the Datastore exists or not.
    govc datastore.info $k8s_secret_default_datastore &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Verified Datastore:" $k8s_secret_default_datastore is present in the inventory.
    else
        echo "[ERROR] Unable to find Datastore:" $k8s_secret_default_datastore.
        exit $ERROR_VC_OBJECT_NOT_FOUND;
    fi

    # Check if the working directory VM folder exists. If not then create this folder
    IFS="/"
    vmFolders=($k8s_secret_node_vms_folder)
    parentFolder=""
    for vmFolder in "${vmFolders[@]}"
    do
        govc folder.info "/$k8s_secret_datacenter/vm/$parentFolder/$vmFolder" &> /dev/null
        if [ $? -eq 0 ]; then
            echo "[INFO] Verified Node VMs Folder:" /$k8s_secret_datacenter/vm/$parentFolder/$vmFolder is present in the inventory.
        else
            echo "Creating folder: " /$k8s_secret_datacenter/vm/$parentFolder/$vmFolder
            govc folder.create "/$k8s_secret_datacenter/vm/$parentFolder/$vmFolder" &> /dev/null
            if [ $? -eq 0 ]; then
                echo "[INFO] Successfully created a new VM Folder:"/$k8s_secret_datacenter/vm/$parentFolder/$vmFolder
            else
                echo "[ERROR] Failed to create a vm folder:" /$k8s_secret_datacenter/vm/$parentFolder/$vmFolder
                exit $ERROR_FOLDER_CREATE;
            fi
        fi
        parentFolder=$parentFolder/$vmFolder
    done

    govc folder.info "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Verified Node VMs Folder:" "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder" is present in the inventory.
    else
        echo "[ERROR] Unable to find VM Folder:" "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder"
        exit $ERROR_VC_OBJECT_NOT_FOUND;
    fi

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
    assign_role_to_user_and_entity $k8s_secret_vcp_username $ROLE_NAME "/" $PROPAGATE

    ROLE_NAME=ReadOnly
    ENTITY="$k8s_secret_datacenter"
    PROPAGATE=false
    assign_role_to_user_and_entity $k8s_secret_vcp_username $ROLE_NAME "$ENTITY" $PROPAGATE

    ROLE_NAME=manage-k8s-volumes
    ENTITY="/$k8s_secret_datacenter/datastore/$k8s_secret_default_datastore"
    PROPAGATE=false
    assign_role_to_user_and_entity $k8s_secret_vcp_username $ROLE_NAME "$ENTITY" $PROPAGATE

    IFS="/"
    vmFolders=($k8s_secret_node_vms_folder)
    parentFolder=""
    ROLE_NAME=manage-k8s-node-vms
    PROPAGATE=true
    for vmFolder in "${vmFolders[@]}"
    do
        ENTITY="/$k8s_secret_datacenter/vm/$parentFolder/$vmFolder"
        assign_role_to_user_and_entity $k8s_secret_vcp_username $ROLE_NAME "$ENTITY" $PROPAGATE
        parentFolder=$parentFolder/$vmFolder
    done

    ROLE_NAME=manage-k8s-node-vms
    ENTITY="/$k8s_secret_datacenter/host/$k8s_secret_node_vms_cluster_or_host"
    PROPAGATE=true
    assign_role_to_user_and_entity $k8s_secret_vcp_username $ROLE_NAME "$ENTITY" $PROPAGATE
else
    echo "Skipping Operations for VCP user. VCP user and Administrator user is same."
fi

kubectl create -f /opt/enable-vcp-scripts/vcp-daemontset.yaml
if [ $? -eq 0 ]; then
    echo "[INFO] Executed kubectl create command to create vcp-daemontset."
else
    echo "[ERROR] 'kubectl create' failed to create vcp-daemonset."
fi

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
