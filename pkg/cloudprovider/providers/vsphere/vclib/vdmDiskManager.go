package vclib

import (
	"golang.org/x/net/context"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type vdmDiskManager struct {
	volumeOptions VolumeOptions
}

// Create implements Disk's Create interface
// Contains implementation of virtualDiskManager based Provisioning
func (vdmDisk vdmDiskManager) Create(ctx context.Context, diskPath string, ds Datastore) (err error) {
	if vdmDisk.volumeOptions.SCSIControllerType == "" {
		vdmDisk.volumeOptions.SCSIControllerType = LSILogicControllerType
	}
	if vdmDisk.volumeOptions.DiskFormat == "" {
		vdmDisk.volumeOptions.DiskFormat = ThinDiskType
	}
	if !vdmDisk.volumeOptions.VerifyVolumeOptions() {
		glog.Error("VolumeOptions verification failed. volumeOptions: ", vdmDisk.volumeOptions)
		return ErrInvalidVolumeOptions
	}
	// Create virtual disk
	diskFormat := diskFormatValidType[vdmDisk.volumeOptions.DiskFormat]
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(ds.Client())
	// Create specification for new virtual disk
	vmDiskSpec := &types.FileBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			AdapterType: vdmDisk.volumeOptions.SCSIControllerType,
			DiskType:    diskFormat,
		},
		CapacityKb: int64(vdmDisk.volumeOptions.CapacityKB),
	}
	task, err := virtualDiskManager.CreateVirtualDisk(ctx, diskPath, ds.datacenter.Datacenter, vmDiskSpec)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", diskPath, err)
		return err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", diskPath, err)
		return err
	}
	return nil
}
