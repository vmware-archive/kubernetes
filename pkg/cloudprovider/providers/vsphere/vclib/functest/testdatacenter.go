package main

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
)

const (
	Username       = "Administrator@vsphere.local"
	Password       = "Admin!23"
	VCenterIP      = "10.160.135.171"
	Port           = "443"
	Insecure       = true
	DatacenterName = "vcqaDC"
)

var vSphereConnection = vclib.VSphereConnection{
	Username: Username,
	Password: Password,
	Hostname: VCenterIP,
	Port:     Port,
	Insecure: Insecure,
}

var dc *vclib.Datacenter
var vmMaster *vclib.VirtualMachine
var vmNode *vclib.VirtualMachine

func main() {
	err := vSphereConnection.Connect()
	if err != nil {
		glog.Errorf("Failed to connect to VC with err: %v", err)
	}
	fmt.Printf("Successfully connected to VC\n")
	fmt.Printf("===============================================\n")
	if vSphereConnection.GoVmomiClient == nil {
		glog.Errorf("vSphereConnection.GoVmomiClient is not set after a successful connect to VC")
	}

	//Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Datacenter functions
	dc, err = vclib.GetDatacenter(ctx, &vSphereConnection, DatacenterName)

	getVMByUUIDTest(ctx, "423787da-df6c-7306-0518-660397085b6f")
	fmt.Printf("===============================================\n")

	vmMaster, err = getVMByPathTest(ctx, "/vcqaDC/vm/kubernetes/master")
	vmNode, err = getVMByPathTest(ctx, "/vcqaDC/vm/kubernetes/node1")
	fmt.Printf("===============================================\n")

	ds1, err := getDatastoreByPathTest(ctx, "[vsanDatastore] kubevols/redis-master.vmdk")
	fmt.Printf("===============================================\n")

	ds2, err := getDatastoreByNameTest(ctx, "sharedVmfs-0")
	fmt.Printf("===============================================\n")

	f, err := getFolderByPathTest(ctx, "/vcqaDC/vm/kubernetes")
	fmt.Printf("===============================================\n")

	vmList, err := f.GetVirtualMachines(ctx)
	fmt.Printf("vmList: %+v\n", vmList)
	fmt.Printf("===============================================\n")

	getVMMoListTest(ctx, []*vclib.VirtualMachine{vmMaster, vmNode}, []string{"name", "summary"})
	fmt.Printf("===============================================\n")

	getDatastoreMoListTest(ctx, []*vclib.Datastore{ds1, ds2}, []string{"name", "summary"})
	fmt.Printf("===============================================\n")

	// Virtual Machine functions
	attachDiskTest(ctx, "[vsanDatastore] kubevols/redis-slave.vmdk", &vclib.VolumeOptions{SCSIControllerType: vclib.PVSCSIControllerType})
	fmt.Printf("===============================================\n")

	isDiskAttachedTest(ctx, "[vsanDatastore] kubevols/redis-master.vmdk")
	fmt.Printf("===============================================\n")

	detachDiskTest(ctx, "[vsanDatastore] kubevols/redis-slave.vmdk")
	fmt.Printf("===============================================\n")

	getResourcePoolTest(ctx)
	fmt.Printf("===============================================\n")

	getAllAccessibleDatastoresTest(ctx)
	fmt.Printf("===============================================\n")

	// diskUUID, err := vmNode.GetVirtualDiskPage83Data(ctx, "[vsanDatastore] kubevols")
	// fmt.Printf("Disk with diskPath: [vsanDatastore] kubevols is %q on VM: %q\n", diskUUID, vmNode.Name())
}

// isDiskAttachedTest checks if disk is attached to the VM.
func isDiskAttachedTest(ctx context.Context, diskPath string) {
	attached, err := vmNode.IsDiskAttached(ctx, diskPath)
	if err != nil {
		glog.Errorf("Failed to check whether disk is attached. err: %s", err)
	}
	if attached {
		fmt.Printf("Disk with diskPath: %q is attached to VM: %q\n", diskPath, vmNode.Name())
	} else {
		fmt.Printf("Disk with diskPath: %q is not attached to VM: %q\n", diskPath, vmNode.Name())
	}
}

func attachDiskTest(ctx context.Context, vmDiskPath string, volumeOptions *vclib.VolumeOptions) {
	diskUUID, err := vmNode.AttachDisk(ctx, vmDiskPath, volumeOptions)
	if err != nil {
		glog.Errorf("Failed to attach disk with path: %q for VM: %q. err: %s", vmDiskPath, vmNode.Name(), err)
		return
	}
	fmt.Printf("Attached disk with diskPath: %q on VM: %q. DiskUUID is %q\n", vmDiskPath, vmNode.Name(), diskUUID)
}

func detachDiskTest(ctx context.Context, vmDiskPath string) {
	err := vmNode.DetachDisk(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("Failed to detach disk with path: %q for VM: %q. err: %s", vmDiskPath, vmNode.Name(), err)
		return
	}
	fmt.Printf("Detached disk with diskPath: %q on VM: %q\n", vmDiskPath, vmNode.Name())
}

func getResourcePoolTest(ctx context.Context) {
	resourcePool, err := vmNode.GetResourcePool(ctx)
	if err != nil {
		glog.Errorf("Failed to get resource pool for VM: %q. err: %s", vmNode.Name(), err)
		return
	}
	fmt.Printf("Resource pool for VM: %q is %+v\n", vmNode.Name(), resourcePool)
}

func getAllAccessibleDatastoresTest(ctx context.Context) {
	dsObjList, err := vmNode.GetAllAccessibleDatastores(ctx)
	if err != nil {
		glog.Errorf("Failed to get all accessible datastores for VM: %q. err: %s", vmNode.Name(), err)
		return
	}
	fmt.Printf("Accessible datastores are %+v for VM:%q\n", dsObjList, vmNode.Name())
}

func getVMByUUIDTest(ctx context.Context, vmUUID string) {
	vm, err := dc.GetVMByUUID(ctx, vmUUID)
	if err != nil {
		glog.Errorf("Failed to get VM from vmUUID: %q with err: %v", vmUUID, err)
	}
	fmt.Printf("VM details are %v\n", vm)
}

func getVMByPathTest(ctx context.Context, vmPath string) (*vclib.VirtualMachine, error) {
	vm, err := dc.GetVMByPath(ctx, vmPath)
	if err != nil {
		glog.Errorf("Failed to get VM from vmPath: %q with err: %v", vmPath, err)
		return nil, err
	}
	fmt.Printf("VM details are %v\n", vm)
	return vm, nil
}

func getDatastoreByPathTest(ctx context.Context, vmDiskPath string) (*vclib.Datastore, error) {
	ds, err := dc.GetDatastoreByPath(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("Failed to get Datastore from vmDiskPath: %q with err: %v", vmDiskPath, err)
		return nil, err
	}
	fmt.Printf("Datastore details are %v\n", ds)
	return ds, nil
}

func getDatastoreByNameTest(ctx context.Context, name string) (*vclib.Datastore, error) {
	ds, err := dc.GetDatastoreByName(ctx, name)
	if err != nil {
		glog.Errorf("Failed to get Datastore from name: %q with err: %v", name, err)
		return nil, err
	}
	fmt.Printf("Datastore details are %v\n", ds)
	return ds, nil
}

func getFolderByPathTest(ctx context.Context, folderPath string) (*vclib.Folder, error) {
	folder, err := dc.GetFolderByPath(ctx, folderPath)
	if err != nil {
		glog.Errorf("Failed to get Datastore from folderPath: %q with err: %v", folderPath, err)
		return nil, err
	}
	fmt.Printf("Folder details are %v\n", folder)
	return folder, nil
}

func getVMMoListTest(ctx context.Context, vmObjList []*vclib.VirtualMachine, properties []string) {
	vmMoList, err := dc.GetVMMoList(ctx, vmObjList, properties)
	if err != nil {
		glog.Errorf("Failed to get VM managed objects with the given properties from the VM objects. vmObjList: %+v, properties: +%v, err: %+v", vmObjList, properties, err)
	}
	for _, vmMo := range vmMoList {
		fmt.Printf("VM name is %q\n", vmMo.Name)
		fmt.Printf("VM summary is %+v\n", vmMo.Summary)
	}
}

func getDatastoreMoListTest(ctx context.Context, dsObjList []*vclib.Datastore, properties []string) {
	dsMoList, err := dc.GetDatastoreMoList(ctx, dsObjList, properties)
	if err != nil {
		glog.Errorf("Failed to get datastore managed objects with the given properties from the datastore objects. vmObjList: %+v, properties: +%v, err: %+v", dsObjList, properties, err)
	}
	for _, dsMo := range dsMoList {
		fmt.Printf("Datastore name is %q\n", dsMo.Name)
		fmt.Printf("Datastore summary is %+v\n", dsMo.Summary)
	}
}
