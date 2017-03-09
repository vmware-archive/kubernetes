# vSphere Volume

  - [Prerequisites](#prerequisites)
  - [Examples](#examples)
    - [Volumes](#volumes)
    - [Persistent Volumes](#persistent-volumes)
    - [Storage Class](#storage-class)
    - [Virtual SAN policy support inside Kubernetes] (#virtual-san-policy-support-inside-kubernetes)

## Prerequisites

- Kubernetes with vSphere Cloud Provider configured.
  For cloudprovider configuration please refer [vSphere getting started guide](http://kubernetes.io/docs/getting-started-guides/vsphere/).

## Examples

### Volumes

  1. Create VMDK.

      First ssh into ESX and then use following command to create vmdk,

      ```shell
      vmkfstools -c 2G /vmfs/volumes/datastore1/volumes/myDisk.vmdk
      ```

  2. Create Pod which uses 'myDisk.vmdk'.

     See example

     ```yaml
        apiVersion: v1
        kind: Pod
        metadata:
          name: test-vmdk
        spec:
          containers:
          - image: gcr.io/google_containers/test-webserver
            name: test-container
            volumeMounts:
            - mountPath: /test-vmdk
              name: test-volume
          volumes:
          - name: test-volume
            # This VMDK volume must already exist.
            vsphereVolume:
              volumePath: "[datastore1] volumes/myDisk"
              fsType: ext4
     ```

     [Download example](vsphere-volume-pod.yaml?raw=true)

     Creating the pod:

     ``` bash
     $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pod.yaml
     ```

     Verify that pod is running:

     ```bash
     $ kubectl get pods test-vmdk
     NAME      READY     STATUS    RESTARTS   AGE
     test-vmdk   1/1     Running   0          48m
     ```

### Persistent Volumes

  1. Create VMDK.

      First ssh into ESX and then use following command to create vmdk,

      ```shell
      vmkfstools -c 2G /vmfs/volumes/datastore1/volumes/myDisk.vmdk
      ```

  2. Create Persistent Volume.

      See example:

      ```yaml
      apiVersion: v1
      kind: PersistentVolume
      metadata:
        name: pv0001
      spec:
        capacity:
          storage: 2Gi
        accessModes:
          - ReadWriteOnce
        persistentVolumeReclaimPolicy: Retain
        vsphereVolume:
          volumePath: "[datastore1] volumes/myDisk"
          fsType: ext4
      ```

      [Download example](vsphere-volume-pv.yaml?raw=true)

      Creating the persistent volume:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pv.yaml
      ```

      Verifying persistent volume is created:

      ``` bash
      $ kubectl describe pv pv0001
      Name:		pv0001
      Labels:		<none>
      Status:		Available
      Claim:
      Reclaim Policy:	Retain
      Access Modes:	RWO
      Capacity:	2Gi
      Message:
      Source:
          Type:	vSphereVolume (a Persistent Disk resource in vSphere)
          VolumePath:	[datastore1] volumes/myDisk
          FSType:	ext4
      No events.
      ```

  3. Create Persistent Volume Claim.

      See example:

      ```yaml
      kind: PersistentVolumeClaim
      apiVersion: v1
      metadata:
        name: pvc0001
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 2Gi
      ```

      [Download example](vsphere-volume-pvc.yaml?raw=true)

      Creating the persistent volume claim:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvc.yaml
      ```

      Verifying persistent volume claim is created:

      ``` bash
      $ kubectl describe pvc pvc0001
      Name:		pvc0001
      Namespace:	default
      Status:		Bound
      Volume:		pv0001
      Labels:		<none>
      Capacity:	2Gi
      Access Modes:	RWO
      No events.
      ```

  3. Create Pod which uses Persistent Volume Claim.

      See example:

      ```yaml
      apiVersion: v1
      kind: Pod
      metadata:
        name: pvpod
      spec:
        containers:
        - name: test-container
          image: gcr.io/google_containers/test-webserver
          volumeMounts:
          - name: test-volume
            mountPath: /test-vmdk
        volumes:
        - name: test-volume
          persistentVolumeClaim:
            claimName: pvc0001
      ```

      [Download example](vsphere-volume-pvcpod.yaml?raw=true)

      Creating the pod:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvcpod.yaml
      ```

      Verifying pod is created:

      ``` bash
      $ kubectl get pod pvpod
      NAME      READY     STATUS    RESTARTS   AGE
      pvpod       1/1     Running   0          48m        
      ```

### Storage Class

  __Note: Here you don't need to create vmdk it is created for you.__
  1. Create Storage Class.

      Example 1:

      ```yaml
      kind: StorageClass
      apiVersion: storage.k8s.io/v1beta1
      metadata:
        name: fast
      provisioner: kubernetes.io/vsphere-volume
      parameters:
          diskformat: zeroedthick
      ```

      [Download example](vsphere-volume-sc-fast.yaml?raw=true)

      You can also specify the datastore in the Storageclass as shown in example 2. The volume will be created on the datastore specified in the storage class.
      This field is optional. If not specified as shown in example 1, the volume will be created on the datastore specified in the vsphere config file used to initialize the vSphere Cloud Provider.

      Example 2:
 
      ```yaml
      kind: StorageClass
      apiVersion: storage.k8s.io/v1beta1
      metadata:
        name: fast
      provisioner: kubernetes.io/vsphere-volume
      parameters:
          diskformat: zeroedthick
          datastore: VSANDatastore
      ```

      [Download example](vsphere-volume-sc-with-datastore.yaml?raw=true)
      Creating the storageclass:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-sc-fast.yaml
      ```

      Verifying storage class is created:

      ``` bash
      $ kubectl describe storageclass fast 
      Name:		fast
      Annotations:	<none>
      Provisioner:	kubernetes.io/vsphere-volume
      Parameters:	diskformat=zeroedthick
      No events.        
      ```

  2. Create Persistent Volume Claim.

      See example:

      ```yaml
      kind: PersistentVolumeClaim
      apiVersion: v1
      metadata:
        name: pvcsc001
        annotations:
          volume.beta.kubernetes.io/storage-class: fast
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 2Gi
      ```

      [Download example](vsphere-volume-pvcsc.yaml?raw=true)

      Creating the persistent volume claim:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvcsc.yaml
      ```

      Verifying persistent volume claim is created:

      ``` bash
      $ kubectl describe pvc pvcsc001
      Name:		pvcsc001
      Namespace:	default
      Status:		Bound
      Volume:		pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Labels:		<none>
      Capacity:	2Gi
      Access Modes:	RWO
      No events.
      ```

      Persistent Volume is automatically created and is bounded to this pvc.

      Verifying persistent volume claim is created:

      ``` bash
      $ kubectl describe pv pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Name:		pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Labels:		<none>
      Status:		Bound
      Claim:		default/pvcsc001
      Reclaim Policy:	Delete
      Access Modes:	RWO
      Capacity:	2Gi
      Message:
      Source:
          Type:	vSphereVolume (a Persistent Disk resource in vSphere)
          VolumePath:	[datastore1] kubevols/kubernetes-dynamic-pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d.vmdk
          FSType:	ext4
      No events.
      ```

      __Note: VMDK is created inside ```kubevols``` folder in datastore which is mentioned in 'vsphere' cloudprovider configuration.
      The cloudprovider config is created during setup of Kubernetes cluster on vSphere.__

  3. Create Pod which uses Persistent Volume Claim with storage class.

      See example:

      ```yaml
      apiVersion: v1
      kind: Pod
      metadata:
        name: pvpod
      spec:
        containers:
        - name: test-container
          image: gcr.io/google_containers/test-webserver
          volumeMounts:
          - name: test-volume
            mountPath: /test-vmdk
        volumes:
        - name: test-volume
          persistentVolumeClaim:
            claimName: pvcsc001
      ```

      [Download example](vsphere-volume-pvcscpod.yaml?raw=true)

      Creating the pod:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvcscpod.yaml
      ```

      Verifying pod is created:

      ``` bash
      $ kubectl get pod pvpod
      NAME      READY     STATUS    RESTARTS   AGE
      pvpod       1/1     Running   0          48m        
      ```

### Virtual SAN policy support inside Kubernetes
####About Virtual SAN Storage Policies####
  Virtual SAN Storage Polices define storage requirements, such as performance and availability for your virtual machines. These policies determine how the virtual machine storage objects are provisioned and allocated within the datastore to guarantee the required level of service. When you enable Virtual SAN on a host cluster, a single Virtual SAN datastore is created and a default storage policy is assigned to the datastore.

  A storage capability is typically represented by a key-value pair, where the key is a specific property that the datastore can offer and the value is a metric that the datastore guarantees for a provisioned object, such as a virtual machine metadata object or a virtual disk. When you know the storage requirements of your virtual machines, you can create a storage policy referencing capabilities that the datastore advertises. You can create several policies to capture different types or classes of requirements.

####Using Virtual SAN Storage Capabilities for storage volume provisioning for containers inside Kubernetes###
  Since Virtual SAN is one of the supported vSphere Storage backends with the vSphere cloud provider, VI Admins will also have the ability to specify custom Virtual SAN Storage Capabilities during dynamic volume provisioning. Inturn these Virtual SAN Policies are assigned to underlying virtual disk. Thus, vSphere Cloud Provider enables policy driven dynamic provisioning of kubernetes persistent volumes. It exposes data services offered by the underlying storage platform such as Virtual SAN at granularity of container volumes and provides applications a complete abstraction of storage infrastructure.

  Below is list of all Virtual SAN storage capabilties supported for storage volume provisioning:
  * `hostFailuresToTolerate`
    * Defines the number of host, disk, or network failures a storage object can tolerate. When the fault tolerance method is mirroring: to tolerate "n" failures, "n+1" copies of the object are created and "2n+1" hosts contributing storage are required (if fault domains are configured, "2n+1" fault domains with hosts contributing storage are required). When the fault tolerance method is erasure coding: to tolerate 1 failure, 4 hosts (or fault domains) are required; and to tolerate 2 failures, 6 hosts (or fault domains) are required. Note: A host which is not part of a fault domain is counted as its own single-host fault domain.
    * Default value: 1, Maximum value: 3.

  * `cacheReservation`
    * Flash capacity reserved as read cache for the storage object. Specified as a percentage of the logical size of the object. To be used only for addressing read performance issues. Reserved flash capacity cannot be used by other objects. Unreserved flash is shared fairly among all objects. It is specified in parts per million.
    * This value is expressed in percentage. Default value: 0, Maximum value: 100.

  * `diskStripes`
    * The number of HDDs across which each replica of storage object is striped. A value higher than 1 may result in better performance (for e.g when flash read cache misses need to get serviced from HDD), but also results in higher used of system resources. 
    * Default value: 1, Maximum value: 12.

  * `objectSpaceReservation`
    * Percentage of the logical size of the storage object that will be reserved (thick provisioning) upon VM provisioning. The rest of the storage object is thin provisioned. 
    * This value is expressed in percentage. Default value: 0, Maximum value: 100.

  * `iopsLimit`
    * Defines upper IOPS limit for a disk. IO rate that has been serviced on a disk will be measured and if the rate exceeds the IOPS limit, IO will be delayed to keep it under the limit. Zero value means no limit.
    * Default value: 0.

  * `forceProvisioning`
    * If this option is "1" the object will be provisioned even if the policy specified in the storage policy is not satisfiable with the resources currently available in the cluster. Virtual SAN will try to bring the object into compliance if and when resources become available.
    * Value can be either O or 1.

  __Note: If you do not apply a storage policy during dynamic provisioning on a VSAN datastore, it will use a default Virtual SAN policy with one number of failures to tolerate, a single disk stripe per object, and a thin-provisioned virtual disk.__

  __Note: Here you don't need to create vmdk it is created for you.__
  1. Create Storage Class.

      Example 1:

      ```yaml
      kind: StorageClass
      apiVersion: storage.k8s.io/v1beta1
      metadata:
        name: fast
      provisioner: kubernetes.io/vsphere-volume
      parameters:
          diskformat: zeroedthick
          hostFailuresToTolerate: "2"
          cachereservation: "20"
      ```
      Here a vmdk will be created with the Virtual SAN capabilities - hostFailuresToTolerate to 2 and cachereservation is 20% read cache reserved for storage object. Also the vmdk will be zeroedthick disk.

      [Download example](vsphere-volume-sc-vsancapabilities.yaml?raw=true)

      You can also specify the datastore in the Storageclass as shown in example 2. The volume will be created on the datastore specified in the storage class.
      This field is optional. If not specified as shown in example 1, the volume will be created on the datastore specified in the vsphere config file used to initialize the vSphere Cloud Provider.

      Example 2:
 
      ```yaml
      kind: StorageClass
      apiVersion: storage.k8s.io/v1beta1
      metadata:
        name: fast
      provisioner: kubernetes.io/vsphere-volume
      parameters:
          diskformat: zeroedthick
          datastore: VSANDatastore
          hostFailuresToTolerate: "2"
          cachereservation: "20"
      ```

      [Download example](vsphere-volume-sc-vsancapabilities-with-datastore.yaml?raw=true)
      Creating the storageclass:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-sc-vsancapabilities.yaml
      ```

      Verifying storage class is created:

      ``` bash
      $ kubectl describe storageclass fast 
      Name:		fast
      Annotations:	<none>
      Provisioner:	kubernetes.io/vsphere-volume
      Parameters:	diskformat=zeroedthick, hostFailuresToTolerate="2", cachereservation="20"
      No events.        
      ```

  2. Create Persistent Volume Claim.

      See example:

      ```yaml
      kind: PersistentVolumeClaim
      apiVersion: v1
      metadata:
        name: pvcsc001
        annotations:
          volume.beta.kubernetes.io/storage-class: fast
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 2Gi
      ```

      [Download example](vsphere-volume-pvcsc.yaml?raw=true)

      Creating the persistent volume claim:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvcsc.yaml
      ```

      Verifying persistent volume claim is created:

      ``` bash
      $ kubectl describe pvc pvcsc001
      Name:		pvcsc001
      Namespace:	default
      Status:		Bound
      Volume:		pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Labels:		<none>
      Capacity:	2Gi
      Access Modes:	RWO
      No events.
      ```

      Persistent Volume is automatically created and is bounded to this pvc.

      Verifying persistent volume claim is created:

      ``` bash
      $ kubectl describe pv pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Name:		pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d
      Labels:		<none>
      Status:		Bound
      Claim:		default/pvcsc001
      Reclaim Policy:	Delete
      Access Modes:	RWO
      Capacity:	2Gi
      Message:
      Source:
          Type:	vSphereVolume (a Persistent Disk resource in vSphere)
          VolumePath:	[datastore1] kubevols/kubernetes-dynamic-pvc-80f7b5c1-94b6-11e6-a24f-005056a79d2d.vmdk
          FSType:	ext4
      No events.
      ```

      __Note: VMDK is created inside ```kubevols``` folder in datastore which is mentioned in 'vsphere' cloudprovider configuration.
      The cloudprovider config is created during setup of Kubernetes cluster on vSphere.__

  3. Create Pod which uses Persistent Volume Claim with storage class.

      See example:

      ```yaml
      apiVersion: v1
      kind: Pod
      metadata:
        name: pvpod
      spec:
        containers:
        - name: test-container
          image: gcr.io/google_containers/test-webserver
          volumeMounts:
          - name: test-volume
            mountPath: /test-vmdk
        volumes:
        - name: test-volume
          persistentVolumeClaim:
            claimName: pvcsc001
      ```

      [Download example](vsphere-volume-pvcscpod.yaml?raw=true)

      Creating the pod:

      ``` bash
      $ kubectl create -f examples/volumes/vsphere/vsphere-volume-pvcscpod.yaml
      ```

      Verifying pod is created:

      ``` bash
      $ kubectl get pod pvpod
      NAME      READY     STATUS    RESTARTS   AGE
      pvpod       1/1     Running   0          48m        
      ```

<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/examples/volumes/vsphere/README.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
