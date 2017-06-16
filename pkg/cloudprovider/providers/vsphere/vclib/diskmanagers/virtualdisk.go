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
	VirtualDiskDeleteOperation = "Delete"
)

// VirtualDiskPVirtualDiskProviderrovider defines interfaces for creating disk
type VirtualDiskProvider interface {
	Create(ctx context.Context, datastore *vclib.Datastore) error
	Delete(ctx context.Context, datastore *vclib.Datastore) error
}

// getDiskManager returns vmDiskManager or vdmDiskManager based on given volumeoptions
func getDiskManager(disk *VirtualDisk, diskOperation string) VirtualDiskProvider {
	var diskProvider VirtualDiskProvider
	switch diskOperation {
	case VirtualDiskDeleteOperation:
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

func (virtualDisk *VirtualDisk) Create(ctx context.Context, datastore *vclib.Datastore) error {
	return getDiskManager(virtualDisk, VirtualDiskCreateOperation).Create(ctx, datastore)
}

func (virtualDisk *VirtualDisk) Delete(ctx context.Context, datastore *vclib.Datastore) error {
	return getDiskManager(virtualDisk, VirtualDiskDeleteOperation).Delete(ctx, datastore)
}
