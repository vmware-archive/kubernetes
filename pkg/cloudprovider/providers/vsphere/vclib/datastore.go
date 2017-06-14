package vclib

import (
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

// Datastore extends the govmomi Datastore object
type Datastore struct {
	*object.Datastore
	datacenter *Datacenter
}

// Disk defines interfaces for creating disk
type Disk interface {
	Create(ctx context.Context, diskPath string, datastore Datastore) error
}

// GetDiskManager returns vmDiskManager or vdmDiskManager based on given volumeoptions
func GetDiskManager(volumeOptions VolumeOptions, vmOptions VMOptions) Disk {
	if volumeOptions.StoragePolicyName != "" || volumeOptions.VSANStorageProfileData != "" || volumeOptions.StoragePolicyID != "" {
		return vmDiskManager{volumeOptions, vmOptions}
	}
	return vdmDiskManager{volumeOptions}
}

// CreateVirtualDisk creates a virtual disk at given diskPath using specified values in the volumeOptions object
func (ds Datastore) CreateVirtualDisk(ctx context.Context, diskPath string, volumeOptions VolumeOptions, vmOptions VMOptions) (err error) {
	return GetDiskManager(volumeOptions, vmOptions).Create(ctx, diskPath, ds)
}

// DeleteVolume deletes a disk at given disk path.
func (ds Datastore) DeleteVolume(ctx context.Context, diskPath string) error {
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(ds.Client())

	// Delete virtual disk
	task, err := virtualDiskManager.DeleteVirtualDisk(ctx, diskPath, nil)
	if err != nil {
		glog.Errorf("Failed to delete virtual disk. err: %v", err)
		return err
	}
	return task.Wait(ctx)
}

// CreateDirectory creates the directory at location specified by directoryPath.
// If the intermediate level folders do not exist, and the parameter createParents is true, all the non-existent folders are created.
// directoryPath must be in the format "[vsanDatastore] kubevols"
func (ds Datastore) CreateDirectory(ctx context.Context, directoryPath string, createParents bool) error {
	fileManager := object.NewFileManager(ds.Client())
	err := fileManager.MakeDirectory(ctx, directoryPath, ds.datacenter.Datacenter, createParents)
	if err != nil {
		glog.Errorf("Cannot create dir: %s. err: %v", directoryPath, err)
		if soap.IsSoapFault(err) {
			soapFault := soap.ToSoapFault(err)
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); ok {
				return ErrFileAlreadyExist
			}
		}
		return err
	}
	glog.V(LogLevel).Infof("Created dir with path as %+q", directoryPath)
	return nil
}

// IsVSANDatastore checks the Datastore object is of the type VSAN
func (ds Datastore) IsVSANDatastore(ctx context.Context) (bool, error) {
	pc := property.DefaultCollector(ds.Client())

	// Convert datastores into list of references
	var dsRefs []types.ManagedObjectReference
	dsRefs = append(dsRefs, ds.Reference())

	// Retrieve summary property for the given datastore
	var dsMorefs []mo.Datastore
	err := pc.Retrieve(ctx, dsRefs, []string{"summary"}, &dsMorefs)
	if err != nil {
		glog.Errorf("Failed to retrieve datastore summary property. err: %v", err)
		return false, err
	}
	for _, ds := range dsMorefs {
		if ds.Summary.Type == VSANDatastoreType {
			return true, nil
		}
	}
	return false, nil
}
