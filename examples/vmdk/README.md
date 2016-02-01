<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should
refer to the docs that go with that version.

<strong>
The latest 1.0.x release of this document can be found
[here](http://releases.k8s.io/release-1.0/examples/vmdk/README.md).

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

# Introduction

This document covers usage of the vmdk plugin for consuming storage managed by
vSphere.

To follow the examples one has to use the official image used to install Kubernetes
top of vSphere. Follow the deployment document for vSphere

# Deployment

For steps to deploy the plugin please see the .

# Create a volume

To create a volume run the followin command on the master node.

```
vmware-vmdk-cli create --datastore=my-datastore --name=my-vmware-vmdk -s 10Gi #my-datastore should already exist across all ESX nodes running the same k8s cluster
```

This will create a VMDK that is not attached to any VM and is ready for the
plugin to attach to the VM on which a Pod requesting the volume is instantiated.

# Define a Pod using a VMDK

The volume created can be consumed by a Pod by addressing the plugin, datastore and VMDK in it's descrition.

```
  apiVersion: v1
  kind: Pod
  metadata:
    name: mysql
    labels:
      name: mysql
  spec:
    containers:
    - resources:
         limits :
      cpu: 0.5
    image: mysql
    name: mysql
    env:
      - name: MYSQL_ROOT_PASSWORD
        # change this
        value: yourpassword
    ports:
      - containerPort: 3306
        name: mysql
    volumeMounts:
        # name must match the volume name below
      - name: mysql-persistent-storage
        # mount path within the container
        mountPath: /var/lib/mysql
   volumes:
      - name: mysql-persistent-storage
         vmdk:
              # This VMDK should already exist.
              vmdkVolume: my-vmware-vmdk;
              dataStore: my-datastore
              fsType: ext4
```

# Using a VMDK as a persistent volume

This is enumerated below to explain the uninitiated how a claim can be setup for a
POD. Please read corresponding documentation for PV and PVC, the only relevant
section here is the specification of the PersistentVolume.

Ref: http://kubernetes.io/v1.0/docs/user-guide/persistent-volumes.html

After creation of the VMDK create a persistent volume in kubernetes using the VMDK.

```
  apiVersion: v1
  kind: PersistentVolume
  metadata:
    name: my-vmware-vmdk
  spec:
    capacity:
      storage: 10Gi
    accessModes:
      - ReadWriteOnce
    persistentVolumeReclaimPolicy: Recycle
    vmdk:
      # This VMDK should already exist.
      vmdkVolume: my-vmware-vmdk;
      dataStore: my-datastore
      fsType: ext4
```

Define a claim.

```
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 8Gi
```

Define a Pod using the Claim.

```
kind: Pod
apiVersion: v1
metadata:
  name: mypod
spec:
  containers:
    - name: myfrontend
      image: dockerfile/nginx
      volumeMounts:
      - mountPath: "/var/www/html"
        name: mypd
  volumes:
    - name: mypd
      persistentVolumeClaim:
        claimName: myclaim
```

# How attach and detach work

When a Pod consuming a VMDK is instantiated, the kubelet will invoke the driver
on the local node where the Pod is to be initiated. The plugin then invokes the
local vmware daemon to attach the already created volume to the VM. The volume
is then handed over the kubelet which then formats a FS is needed and is
consumed by the Pod.

TODO: Ref to the repo hosting the daemon.

When the kubelet decides that the Volume is no longer needed, it will detach
the volume from the node. The plugin will invoke the daemon to detach the
volume.

# HA and VMDK

Kubernetes implements the HA primitives (restart, replication controller), as
long as a volume is attached to a node(VM), it cannot be reattached to a
different node. If the volume is cleanly detached from a node it can be mounted
onto any other VM.

# Deleting a VMDK


To delete a volume run the followin command on the master node.

```
vmware-vmdk-cli delete --datastore=my-datastore --name=my-vmware-vmdk #Fails if attached to a VM.
```




<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/examples/vmdk/README.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
