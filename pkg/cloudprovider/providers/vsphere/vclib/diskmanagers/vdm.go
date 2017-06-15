package diskmanagers

import (
	"golang.org/x/net/context"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
)

type vdmDiskManager struct {
	diskPath      string
	volumeOptions vclib.VolumeOptions
}

// Create implements Disk's Create interface
// Contains implementation of virtualDiskManager based Provisioning
func (vdmDisk vdmDiskManager) Create(ctx context.Context, datastore *vclib.Datastore) (err error) {
	if vdmDisk.volumeOptions.SCSIControllerType == "" {
		vdmDisk.volumeOptions.SCSIControllerType = vclib.LSILogicControllerType
	}
	if vdmDisk.volumeOptions.DiskFormat == "" {
		vdmDisk.volumeOptions.DiskFormat = vclib.ThinDiskType
	}
	if !vdmDisk.volumeOptions.VerifyVolumeOptions() {
		glog.Error("VolumeOptions verification failed. volumeOptions: ", vdmDisk.volumeOptions)
		return vclib.ErrInvalidVolumeOptions
	}
	// Create virtual disk
	diskFormat := vclib.DiskFormatValidType[vdmDisk.volumeOptions.DiskFormat]
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(datastore.Client())
	// Create specification for new virtual disk
	vmDiskSpec := &types.FileBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			AdapterType: vdmDisk.volumeOptions.SCSIControllerType,
			DiskType:    diskFormat,
		},
		CapacityKb: int64(vdmDisk.volumeOptions.CapacityKB),
	}
	task, err := virtualDiskManager.CreateVirtualDisk(ctx, vdmDisk.diskPath, datastore.Datacenter.Datacenter, vmDiskSpec)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", vdmDisk.diskPath, err)
		return err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err: %+v", vdmDisk.diskPath, err)
		return err
	}
	return nil
}

// Delete implements Disk's Delete interface
func (vdmDisk vdmDiskManager) Delete(ctx context.Context, datastore *vclib.Datastore) error {
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(datastore.Client())

	// Delete virtual disk
	task, err := virtualDiskManager.DeleteVirtualDisk(ctx, vdmDisk.diskPath, nil)
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
