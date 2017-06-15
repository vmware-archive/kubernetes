package diskmanagers

import (
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
)

// VirtualDisk is for the Disk Management
type VirtualDisk struct {
	DiskPath      string
	VolumeOptions vclib.VolumeOptions
	VMOptions     vclib.VMOptions
}

// VirtualDisk Operations Const
const (
	VirtualDiskCreateOperation = "Create"
	VirtualDeleteOperation     = "Delete"
)

// VirtualDiskProvider defines interfaces for creating disk
type VirtualDiskProvider interface {
	Create(ctx context.Context, datastore *vclib.Datastore) error
	Delete(ctx context.Context, datastore *vclib.Datastore) error
}

// GetDiskManager returns vmDiskManager or vdmDiskManager based on given volumeoptions
func GetDiskManager(disk *VirtualDisk, diskOperation string) VirtualDiskProvider {
	var diskProvider VirtualDiskProvider
	switch diskOperation {
	case VirtualDeleteOperation:
		diskProvider = virtualDiskManager{disk.DiskPath, disk.VolumeOptions}
	case VirtualDiskCreateOperation:
		if disk.VolumeOptions.StoragePolicyName != "" || disk.VolumeOptions.VSANStorageProfileData != "" || disk.VolumeOptions.StoragePolicyID != "" {
			diskProvider = vmDiskManager{disk.DiskPath, disk.VolumeOptions, disk.VMOptions}
		} else {
			diskProvider = virtualDiskManager{disk.DiskPath, disk.VolumeOptions}
		}
	}
	return diskProvider
}
