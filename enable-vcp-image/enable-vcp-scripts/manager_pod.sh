#!/bin/bash

read_secret_keys() {
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
            exit 1;
        fi
    fi
}

wait_for_role_to_exist() {
    ROLE_NAME=$1
    while true; do
        govc role.ls $ROLE_NAME &> /dev/null
        if [ $? -eq 0 ]; then
            break;
        fi
    done
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
        echo "[ERROR] Failed to add Previledges:["$PREVILEDGES"] to the Role:" $ROLE_NAME
        exit 1;
    fi
}

assign_role_to_user_and_entity() {
    vcp_user=$1
    ROLE_NAME=$2
    ENTITY=$3
    PROPAGATE=$4
    govc permissions.set -principal $vcp_user -propagate=$PROPAGATE -role $ROLE_NAME "$ENTITY" &> /dev/null
    if [ $? -eq 0 ]; then
        echo "[INFO] Role:["$ROLE_NAME"] assigned to the User:["$vcp_user"] on Entity:["$ENTITY"]"
    else
        echo "[ERROR] Failed to Assign Role:["$ROLE_NAME"] to the User:["$vcp_user"] on Entity:["$ENTITY"]"
        exit 1;
    fi
}

# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys

# connect to vCenter using VCP username and password
export GOVC_INSECURE=1
export GOVC_URL='https://'$k8s_secret_vcp_username':'$k8s_secret_vcp_password'@'$k8s_secret_vc_ip':'$k8s_secret_vc_port'/sdk'
error_message=$(govc ls 2>&1 >/dev/null)

if [ $? -eq 1 ]; then
    if [$error_message == "govc: ServerFaultCode: Cannot complete login due to an incorrect user name or password."]; then
        echo "Failed to login to vCenter using VCP Username:" $k8s_secret_vcp_username " and VCP Password:" $k8s_secret_vcp_password
        exit 1;
    fi
fi

# connect to vCenter using VC Admin username and password
export GOVC_URL='https://'$k8s_secret_vc_admin_username':'$k8s_secret_vc_admin_password'@'$k8s_secret_vc_ip':'$k8s_secret_vc_port'/sdk'

# Verify if the Datacenter exists or not.
govc datacenter.info $k8s_secret_datacenter &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Datacenter:" $k8s_secret_datacenter is present in the inventory.
else
    echo "[ERROR] Unable to find Datacenter:" $k8s_secret_datacenter.
    exit 1;
fi

# Verify if the Datastore exists or not.
govc datastore.info $k8s_secret_default_datastore &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Datastore:" $k8s_secret_default_datastore is present in the inventory.
else
    echo "[ERROR] Unable to find Datastore:" $k8s_secret_default_datastore.
    exit 1;
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
            exit 1;
        fi
    fi
    parentFolder=$parentFolder/$vmFolder
done

govc folder.info "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder" &> /dev/null
if [ $? -eq 0 ]; then
    echo "[INFO] Verified Node VMs Folder:" "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder" is present in the inventory.
else
    echo "[ERROR] Unable to find VM Folder:" "/$k8s_secret_datacenter/vm/$k8s_secret_node_vms_folder"
    exit 1;
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
