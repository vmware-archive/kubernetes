package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib/diskmanagers"
	"k8s.io/kubernetes/pkg/volume"
)

// constants
const (
	Username       = "Administrator@vsphere.local"
	Password       = "Admin!23"
	VCenterIP      = "10.162.26.135"
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
var datastoreDirectoryIDMap = make(map[string]string)

func main() {
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := vSphereConnection.Connect(ctx)
	if err != nil {
		glog.Errorf("Failed to connect to VC with err: %v", err)
	}
	fmt.Printf("Successfully connected to VC\n")
	fmt.Printf("===============================================\n")
	if vSphereConnection.GoVmomiClient == nil {
		glog.Errorf("vSphereConnection.GoVmomiClient is not set after a successful connect to VC")
	}

	dc, err := vclib.GetDatacenter(ctx, &vSphereConnection, DatacenterName)
	if err != nil {
		glog.Errorf("Failed to connect to Datacenter: %+v", err)
		return
	}
	volSizeBytes := int64(1073741824)
	// vSphere works with kilobytes, convert to KiB with rounding up
	volSizeKB := int(volume.RoundUpSize(volSizeBytes, 1024))
	volumeOptions := vclib.VolumeOptions{
		CapacityKB:        volSizeKB,
		Name:              "testvmdk12",
		DiskFormat:        "thin",
		Datastore:         "vsanDatastore",
		StoragePolicyName: "gold",
	}

	ds, err := dc.GetDatastoreByName(ctx, volumeOptions.Datastore)
	if err != nil {
		glog.Errorf("Failed to connect to Datastore: %+v", err)
		return
	}

	kubeVolsPath := filepath.Clean(ds.Path("kubevols")) + "/"
	err = ds.CreateDirectory(ctx, kubeVolsPath, false)
	if err != nil && err != vclib.ErrFileAlreadyExist {
		glog.Errorf("Cannot create dir %#v. err %s", kubeVolsPath, err)
		return
	}

	volumePath := kubeVolsPath + volumeOptions.Name + ".vmdk"
	var vmOptions *vclib.VMOptions
	vmOptions, err = setVMOptions(ctx, dc)
	if err != nil {
		glog.Errorf("Failed to set VM options requires to create a vsphere volume. err: %+v", err)
	}

	disk := diskmanagers.VirtualDisk{
		DiskPath:      volumePath,
		VolumeOptions: &volumeOptions,
		VMOptions:     vmOptions,
	}
	canonicalVolumePath, err := disk.Create(ctx, ds)

	// If the datastore doesn't exist in datastoreDirectoryMap
	if _, ok := datastoreDirectoryIDMap[volumeOptions.Datastore]; !ok {
		if canonicalVolumePath == volumePath {
			canonicalVolumePath, err = createDummyVirtualDisk(ctx, ds)
			if err != nil {
				glog.Errorf("Failed to create a dummy vsphere volume. err: %+v", err)
			}
			deleteDummyVirtualDisk(ctx, ds)
		}
		diskPath := vclib.GetPathFromVMDiskPath(canonicalVolumePath)
		if diskPath == "" {
			glog.Errorf("Failed to parse canonicalVolumePath: %s", canonicalVolumePath)
		}
		datastoreDirectoryIDMap[volumeOptions.Datastore] = strings.Split(strings.TrimSpace(diskPath), "/")[0]
	}
	canonicalVolumePath = strings.Replace(canonicalVolumePath, "kubevols", datastoreDirectoryIDMap[volumeOptions.Datastore], 1)
	if filepath.Base(volumeOptions.Datastore) != volumeOptions.Datastore {
		// If datastore is within cluster, add cluster path to the volumePath
		canonicalVolumePath = strings.Replace(canonicalVolumePath, filepath.Base(volumeOptions.Datastore), volumeOptions.Datastore, 1)
	}
	fmt.Printf("canonicalVolumePath is %+q\n", canonicalVolumePath)
	fmt.Printf("===============================================\n")
}

func setVMOptions(ctx context.Context, dc *vclib.Datacenter) (*vclib.VMOptions, error) {
	var vmOptions vclib.VMOptions
	vm, err := dc.GetVMByPath(ctx, "kubernetes/master")
	if err != nil {
		return nil, err
	}
	resourcePool, err := vm.GetResourcePool(ctx)
	if err != nil {
		return nil, err
	}
	folder, err := dc.GetFolderByPath(ctx, "kubernetes")
	if err != nil {
		return nil, err
	}
	vmOptions.VMFolder = folder
	vmOptions.VMResourcePool = resourcePool
	return &vmOptions, nil
}

func createDummyVirtualDisk(ctx context.Context, ds *vclib.Datastore) (string, error) {
	kubeVolsPath := filepath.Clean(ds.Path("kubevols")) + "/"
	dummyDiskVolPath := kubeVolsPath + "kube-dummyDisk.vmdk"
	dummyDiskVolOptions := vclib.VolumeOptions{
		SCSIControllerType: vclib.LSILogicControllerType,
		DiskFormat:         "thin",
		CapacityKB:         20480,
	}
	dummyDisk := diskmanagers.VirtualDisk{
		DiskPath:      dummyDiskVolPath,
		VolumeOptions: &dummyDiskVolOptions,
		VMOptions:     &vclib.VMOptions{},
	}
	dummyDiskCanonicalVolPath, err := dummyDisk.Create(ctx, ds)
	if err != nil {
		glog.Warningf("Failed to create a dummy vsphere volume with volumeOptions: %+v on datastore: %+v. err: %+v", dummyDiskVolOptions, ds, err)
		return "", err
	}
	return dummyDiskCanonicalVolPath, nil
}

func deleteDummyVirtualDisk(ctx context.Context, ds *vclib.Datastore) error {
	kubeVolsPath := filepath.Clean(ds.Path("kubevols")) + "/"
	dummyDiskVolPath := kubeVolsPath + "kube-dummyDisk.vmdk"
	disk := diskmanagers.VirtualDisk{
		DiskPath:      dummyDiskVolPath,
		VolumeOptions: &vclib.VolumeOptions{},
		VMOptions:     &vclib.VMOptions{},
	}
	err := disk.Delete(ctx, ds)
	if err != nil {
		glog.Errorf("Failed to delete dummy vsphere volume with dummyDiskVolPath: %s. err: %+v", dummyDiskVolPath, err)
		return err
	}
	return nil
}
