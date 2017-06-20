package diskmanagers

import (
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
	task, err := vdm.CreateVirtualDisk(ctx, diskManager.diskPath, datastore.Datacenter.Datacenter, vmDiskSpec)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", diskManager.diskPath, err)
		return err
	}
	err = task.Wait(ctx)
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

	// Delete virtual disk
	task, err := virtualDiskManager.DeleteVirtualDisk(ctx, diskManager.diskPath, datastore.Datacenter.Datacenter)
	if err != nil {
		glog.Errorf("Failed to delete virtual disk. err: %v", err)
		return err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to delete virtual disk. err: %v", err)
		return err
	}
	return nil
}
