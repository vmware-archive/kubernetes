package vclib

import (
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"golang.org/x/net/context"
)

// VirtualDisk is for the Disk Management
type VirtualDisk struct {
	DiskPath      string
	VolumeOptions VolumeOptions
	VMOptions     VMOptions
}

// Disk defines interfaces for creating disk
type Disk interface {
	Create(ctx context.Context, datastore *Datastore) error
}

// GetDiskManager returns vmDiskManager or vdmDiskManager based on given volumeoptions
func GetDiskManager(disk *VirtualDisk) Disk {
	if disk.VolumeOptions.StoragePolicyName != "" || disk.VolumeOptions.VSANStorageProfileData != "" || disk.VolumeOptions.StoragePolicyID != "" {
		return vmDiskManager{disk.DiskPath, disk.VolumeOptions, disk.VMOptions}
	}
	return vdmDiskManager{disk.DiskPath, disk.VolumeOptions}
}

// CreateVirtualDisk creates a virtual disk at given diskPath using specified values in the volumeOptions object
func (disk *VirtualDisk) CreateVirtualDisk(ctx context.Context, datastore *Datastore) (err error) {
	return GetDiskManager(disk).Create(ctx, datastore)
}

// DeleteVolume deletes a disk at given disk path.
func (disk *VirtualDisk) DeleteVolume(ctx context.Context, datastore *Datastore) error {
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(datastore.Client())

	// Delete virtual disk
	task, err := virtualDiskManager.DeleteVirtualDisk(ctx, disk.DiskPath, nil)
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
