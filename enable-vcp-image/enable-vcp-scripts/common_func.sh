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
        echo "[ERROR] Failed to add Previledges:['$PREVILEDGES'] to the Role:" $ROLE_NAME
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
        echo "[INFO] Role:["$ROLE_NAME"] assigned to the User:['$vcp_user'] on Entity:['$ENTITY']"
    else
        echo "[ERROR] Failed to Assign Role:["$ROLE_NAME"] to the User:['$vcp_user'] on Entity:['$ENTITY']"
        exit 1;
    fi
}