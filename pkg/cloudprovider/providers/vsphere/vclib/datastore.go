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

type DataStore struct {
	*object.Datastore
}

// Creates a directory using the specified name. If the intermediate level folders do not exist,
// and the parameter createParents is true, all the non-existent folders are created.
func (ds DataStore) CreateDirectory(ctx context.Context, directoryPath string, createParents bool) error {
	fileManager := object.NewFileManager(ds.Client())
	err := fileManager.MakeDirectory(ctx, directoryPath, nil, createParents)
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
	glog.V(LOG_LEVEL).Infof("Created dir with path as %+q", directoryPath)
	return nil
}

// Create a virtual disk at given diskPath using specified values in the volumeOptions object
func (ds DataStore) CreateVirtualDisk(ctx context.Context, diskPath string, volumeOptions VolumeOptions) (err error) {
	if !volumeOptions.VerifyVolumeOptions() {
		glog.Error("VolumeOptions verification failed")
		return ErrInvalidVolumeOptions
	}
	// Create virtual disk
	diskFormat := diskFormatValidType[volumeOptions.DiskFormat]
	// Create a virtual disk manager
	virtualDiskManager := object.NewVirtualDiskManager(ds.Client())
	// Create specification for new virtual disk
	vmDiskSpec := &types.FileBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			AdapterType: LSILogicControllerType,
			DiskType:    diskFormat,
		},
		CapacityKb: int64(volumeOptions.CapacityKB),
	}
	task, err := virtualDiskManager.CreateVirtualDisk(ctx, diskPath, nil, vmDiskSpec)
	if err != nil {
		glog.Errorf("Failed to create virtual disk: %s. err %v", diskPath, err)
		return err
	}
	return task.Wait(ctx)
}

// Creates a virtual disk with the policy configured to the disk.
// A call to this function is made only when a user specifies VSAN storage capabilties in the storage class definition.
func (ds DataStore) CreateVirtualDiskWithPolicy(ctx context.Context, diskPath string, virtualMachine VirtualMachine, diskControllerType string, volumeOptions VolumeOptions) error {

	disk, _, err := CreateDiskSpec(ctx, virtualMachine, ds.Reference(), diskPath, diskControllerType, volumeOptions)
	if err != nil {
		glog.Errorf("Failed to create Disk Spec. err: %v", err)
		return err
	}
	// Reconfigure VM
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{}
	deviceConfigSpec := &types.VirtualDeviceConfigSpec{
		Device:        disk,
		Operation:     types.VirtualDeviceConfigSpecOperationAdd,
		FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
	}
	storageProfileSpec := &types.VirtualMachineDefinedProfileSpec{}
	// Is PBM storage policy ID is present, set the storage spec profile ID,
	// else, set raw the VSAN policy string.
	if volumeOptions.StoragePolicyID != "" {
		storageProfileSpec.ProfileId = volumeOptions.StoragePolicyID
	} else if volumeOptions.VSANStorageProfileData != "" {
		storageProfileSpec.ProfileId = ""
		storageProfileSpec.ProfileData = &types.VirtualMachineProfileRawData{
			ExtensionKey: "com.vmware.vim.sps",
			ObjectData:   volumeOptions.VSANStorageProfileData,
		}
	}
	deviceConfigSpec.Profile = append(deviceConfigSpec.Profile, storageProfileSpec)
	virtualMachineConfigSpec.DeviceChange = append(virtualMachineConfigSpec.DeviceChange, deviceConfigSpec)
	task, err := virtualMachine.Reconfigure(ctx, virtualMachineConfigSpec)
	if err != nil {
		glog.Errorf("Failed to reconfigure the VM with the disk with err - %v.", err)
		return err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to reconfigure the VM with the disk with err - %v.", err)
		return err
	}
	return nil
}

//  Deletes a disk at given disk path.
func (ds DataStore) DeleteVolume(ctx context.Context, diskPath string) error {
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

// Check the Datastore object is of the type VSAN
func (ds DataStore) IsVSANDatastore(ctx context.Context) (bool, error) {
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
