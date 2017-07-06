package diskmanagers

import (
	"time"

	"golang.org/x/net/context"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
)

// virtualDiskManager implements VirtualDiskProvider Interface for creating and deleting volume using VirtualDiskManager
type virtualDiskManager struct {
	diskPath      string
	volumeOptions *vclib.VolumeOptions
}

// Create implements Disk's Create interface
// Contains implementation of virtualDiskManager based Provisioning
func (diskManager virtualDiskManager) Create(ctx context.Context, datastore *vclib.Datastore) (err error) {
	if diskManager.volumeOptions.SCSIControllerType == "" {
		diskManager.volumeOptions.SCSIControllerType = vclib.LSILogicControllerType
	}
	// Create virtual disk
	diskFormat := vclib.DiskFormatValidType[diskManager.volumeOptions.DiskFormat]
	// Create a virtual disk manager
	vdm := object.NewVirtualDiskManager(datastore.Client())
	// Create specification for new virtual disk
	vmDiskSpec := &types.FileBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			AdapterType: diskManager.volumeOptions.SCSIControllerType,
			DiskType:    diskFormat,
		},
		CapacityKb: int64(diskManager.volumeOptions.CapacityKB),
	}
	requestTime := time.Now()
	// Create virtual disk
	task, err := vdm.CreateVirtualDisk(ctx, diskManager.diskPath, datastore.Datacenter.Datacenter, vmDiskSpec)
	if err != nil {
		vclib.RecordvSphereMetric(vclib.APICreateVolume, requestTime, err)
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", diskManager.diskPath, err)
		return err
	}
	err = task.Wait(ctx)
	vclib.RecordvSphereMetric(vclib.APICreateVolume, requestTime, err)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", diskManager.diskPath, err)
		return err
	}
	return nil
}

// Delete implements Disk's Delete interface
func (diskManager virtualDiskManager) Delete(ctx context.Context, datastore *vclib.Datastore) error {
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(datastore.Client())
	diskPath := vclib.RemoveClusterFromVDiskPath(diskManager.diskPath)
	requestTime := time.Now()
	// Delete virtual disk
	task, err := virtualDiskManager.DeleteVirtualDisk(ctx, diskPath, datastore.Datacenter.Datacenter)
	if err != nil {
		glog.Errorf("Failed to delete virtual disk. err: %v", err)
		vclib.RecordvSphereMetric(vclib.APIDeleteVolume, requestTime, err)
		return err
	}
	err = task.Wait(ctx)
	vclib.RecordvSphereMetric(vclib.APIDeleteVolume, requestTime, err)
	if err != nil {
		glog.Errorf("Failed to delete virtual disk. err: %v", err)
		return err
	}
	return nil
}
