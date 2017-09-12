# Enabling vSphere Cloud Provider with automation script

To enable the vSphere Cloud Provider on Kubernetes Cluster, users need to follow steps mentioned [here](https://kubernetes.io/docs/getting-started-guides/vsphere/#enable-vsphere-cloud-provider).

Some of the steps, for example, creating roles and privileges assignment, enabling advanced properties on the Kubernetes nodes etc. requires vSphere administration skill. To help developers quickly get started with enabling vSphere Cloud Provider we have automated most of the vCenter administration steps. To know detail about automation architecture and design please visit this [link](https://github.com/vmware/kubernetes/issues/224).

Automation takes care of the following tasks: 

 * Create working directory VM folders for Kubernetes node VMs if not present.
 * Move node VMs to the working directory VM folder.
 * Create roles and assign privilege to vCenter entities.
 * Enable disk.enableUUID on Kubernetes node VMs.
 * Rename Kubernets node VM names to registered node names.
 * Create vSphere.conf file.
 * Add required flags to the pod manifest files of API server, Controller Manager and Kubelet to enable vSphere Cloud Provider.
 * Reload kubelet service unit and restart Kubelet service on all nodes to apply the configuration.

## How to kick off auto configuration
Automation script files are located at [https://github.com/vmware/kubernetes/tree/enable-vcp-uxi](https://github.com/vmware/kubernetes/tree/enable-vcp-uxi). 

Scripts are bundled in the docker image located at https://hub.docker.com/r/cnastorage/enablevcp

Following are the Prerequisites for this automation.
 
 * We need a vCenter admin username and password.
 * Separate user for vSphere Cloud Provider needs to be pre-created on the vCenter. This Step is optional but recommended.
 
Let's get started with how to use these automation scripts.

**The first step** is to download following YAML files :

```bash
wget https://raw.githubusercontent.com/vmware/kubernetes/enable-vcp-uxi/enable-vsphere-cloud-provider.yaml

wget https://raw.githubusercontent.com/vmware/kubernetes/enable-vcp-uxi/vcp_namespace_account_and_roles.yaml

wget https://raw.githubusercontent.com/vmware/kubernetes/enable-vcp-uxi/vcp_secret.yaml
```

**The second step** is to fill in the details in the `vcp_secret.yaml` file.  All fields in the `vcp_secret.yaml` file are mandatory and cannot be empty. Most of them are self-explanatory.

For usernames and passwords in the secret file make sure you encode them with base64 as mentioned below.

```bash
$ echo -n 'Administrator@vsphere.local' | base64
QWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs

$ echo -n 'password' | base64
cGFzc3dvcmQ=
```
**Note:** If you want to use administrator user as vSphere Cloud Provider user, fill in the same value for `vc_admin_username` and `vcp_username` and corresponding passwords.

Fields mentioned under the `stringData` section should not be encoded.

**The third step** is to deploy secret volume, manager Pod and daemon sets.

Deploy them in the following sequence.

```bash
$ kubectl create -f vcp_namespace_account_and_roles.yaml 
namespace "vmware" created
serviceaccount "vcpsa" created
clusterrolebinding "sa-vmware-default-binding" created
clusterrolebinding "sa-vmware-vcpsa-binding" created

$ kubectl create --save-config -f vcp_secret.yaml
secret "vsphere-cloud-provider-secret" created

$ kubectl create -f enable-vsphere-cloud-provider.yaml 
pod "vcp-manager" created
```

That's it! The vSphere Cloud Provider should be enabled on the cluster in few minutes.

## How to monitor the configuration progress

Verify if configuration Pods are running.

```bash
kubectl get pods --namespace=vmware
NAME                   READY     STATUS    RESTARTS   AGE
vcp-daementset-3cgss   1/1       Running   0          6m
vcp-daementset-b0sn2   1/1       Running   0          6m
vcp-daementset-dc109   1/1       Running   0          6m
vcp-daementset-nzsvb   1/1       Running   0          6m
vcp-daementset-q356x   1/1       Running   0          6m
vcp-manager            1/1       Running   0          7m
```

You can see the logs on for these pods using

```
kubectl logs <pod-name> --namespace=vmware
```

The progress can also be monitored using the Third Party Resource - `VcpSummary`, as below.

```bash
$ kubectl get VcpSummary --namespace=vmware -o json
{
    "apiVersion": "v1",
    "items": [
        {
            "apiVersion": "vmware.com/v1",
            "kind": "VcpSummary",
            "metadata": {
                "annotations": {
                    "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"vmware.com/v1\",\"kind\":\"VcpSummary\",\"metadata\":{\"annotations\":{},\"name\":\"vcpinstallstatus\",\"namespace\":\"vmware\"},\"spec\":{\"nodes_being_configured\":\"2\",\"nodes_failed_to_configure\":\"0\",\"nodes_in_phase1\":\"0\",\"nodes_in_phase2\":\"0\",\"nodes_in_phase3\":\"0\",\"nodes_in_phase4\":\"0\",\"nodes_in_phase5\":\"1\",\"nodes_in_phase6\":\"0\",\"nodes_in_phase7\":\"1\",\"nodes_sucessfully_configured\":\"3\",\"total_number_of_nodes\":\"5\"}}\n"
                },
                "creationTimestamp": "2017-08-22T02:36:03Z",
                "name": "vcpinstallstatus",
                "namespace": "vmware",
                "resourceVersion": "126753",
                "selfLink": "/apis/vmware.com/v1/namespaces/vmware/vcpsummaries/vcpinstallstatus",
                "uid": "a4426d3b-86e2-11e7-96b5-005056803917"
            },
            "spec": {
                "nodes_being_configured": "2",
                "nodes_failed_to_configure": "0",
                "nodes_in_phase1": "0",
                "nodes_in_phase2": "0",
                "nodes_in_phase3": "0",
                "nodes_in_phase4": "0",
                "nodes_in_phase5": "1",
                "nodes_in_phase6": "0",
                "nodes_in_phase7": "1",
                "nodes_sucessfully_configured": "3",
                "total_number_of_nodes": "5"
            }
        }
    ],
    "kind": "List",
    "metadata": {},
    "resourceVersion": "",
    "selfLink": ""
}
```
**Note:** During execution of above command, if you encounter you are not able to connect to the Kubernetes cluster, wait for some time. The master node may be restarting API server during that time.

Here when you see `nodes_sucessfully_configured` is equal to the `total_number_of_nodes`, all nodes are configured with the vSphere Cloud Provider.

Here is the description about the phases mentioned in the above JSON.

* Phase 1 - Validation
* Phase 2 - Node VM vCenter Configuration.
* Phase 3 - Move VM to the Working Directory. Rename VM to match with Node Name.
* Phase 4 - Validate and backup existing node configuration
* Phase 5 - Create vSphere.conf file.
* Phase 6 - Update pod manifest and service configuration files.
* Phase 7 - Reload systemd unit files and Restart Kubelet Service
* Phase 8 - Complete

## How to clean up configuration resources.

After the enabling vSphere Cloud Provider using this deployment, if you wish to keep configuration pods, secret volumes service account and role bindings, there is no harm. When new Kubernetes node joins the cluster, a new Daemon Pod will be created on that node and configuration will be applied to that node.

if you want to perform the cleanup, execute following commands in sequence.

```bash
kubectl delete pod vcp-manager --namespace vmware

kubectl delete daemonset vcp-daementset --namespace vmware

kubectl delete ThirdPartyResources vcp-status.vmware.com

kubectl delete ThirdPartyResources vcp-summary.vmware.com

kubectl delete secret vsphere-cloud-provider-secret --namespace=vmware

kubectl delete serviceaccount vcpsa --namespace=vmware

kubectl delete clusterrolebinding sa-vmware-default-binding  sa-vmware-vcpsa-binding

kubectl delete namespace vmware

```

## How to roll back the node VM configuration

In case if the script fails to configure some of the nodes, and you want to revert back the original configuration on the nodes which are successfully configured, we have saved existing node configurations and that can be roll backed.

Pre-requisites
 * All nodes must be up and running.
 * Configuration should be either finished or failed, and should not be in the progress.

To perform roll back execute following commands.

Open `vcp_secret.yaml` file locate the switch for roll back and switch it on and make sure to save the file.

```
enable_roll_back_switch: "on"
```

Delete existing configuration resources.
```bash
kubectl delete pod vcp-manager --namespace vmware
kubectl delete daemonset vcp-daementset --namespace vmware
kubectl delete ThirdPartyResources vcp-status.vmware.com
kubectl delete ThirdPartyResources vcp-summary.vmware.com
kubectl delete secret vsphere-cloud-provider-secret --namespace=vmware
```
Re-create Secret and vcp-manager and daemon set pod.

```
kubectl create --save-config -f vcp_secret.yaml
kubectl create -f enable-vsphere-cloud-provider.yaml 
```

Roll back progress can be monitored from the logs on the Daemon Pods. Once roll back is finished, you can clean up the configuration resources as mentioned above.

## **Support**

For quick support please join VMware Code Slack ([#kubernetes](https://vmwarecode.slack.com/messages/kubernetes/)) and post your question.

If you identify any issues/problems using this script, you can create an issue in our repository - [VMware Kubernetes] (https://github.com/vmware/kubernetes).