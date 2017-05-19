package vclib

import (
	"errors"
	"strings"

	"fmt"
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"golang.org/x/net/context"
)

type DataCenter struct {
	*object.Datacenter
}

// find and return VM for the given VM UUID
func (dc DataCenter) GetVMByUUID(ctx context.Context, vmUUID string) (VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	vmUUID = strings.ToLower(strings.TrimSpace(vmUUID))
	svm, err := s.FindByUuid(ctx, dc.Datacenter, vmUUID, true, nil)
	if err != nil {
		glog.Errorf("Failed to find VM by UUID. VM UUID: %s, err: %v", vmUUID, err)
		return nil, err
	}
	virtualMachine := VirtualMachine{object.NewVirtualMachine(dc.Client(), svm.Reference())}
	return virtualMachine, nil
}

// Returns VM located at vmpath - VMPath should have folder path and VM Name
func (dc DataCenter) GetVMByPath(ctx context.Context, vmpath string) (VirtualMachine, error) {
	finder := getFinder(dc)
	vm, err := finder.VirtualMachine(ctx, vmpath)
	if err != nil {
		glog.Errorf("Failed to find VM by Path. VM Path: %s, err: %v", vmpath, err)
		return nil, err
	}
	virtualMachine := VirtualMachine{vm}
	return virtualMachine, nil
}

// Return datastore for given VM Disk
func (dc DataCenter) GetDataStoreByPath(ctx context.Context, vmDiskPath string) (DataStore, error) {
	datastorePathObj := new(object.DatastorePath)
	isSuccess := datastorePathObj.FromString(vmDiskPath)
	if !isSuccess {
		glog.Errorf("Failed to parse vmDiskPath: %s", vmDiskPath)
		return nil, errors.New("Failed to parse vmDiskPath")
	}
	finder := getFinder(dc)
	ds, err := finder.Datastore(ctx, datastorePathObj.Datastore)
	if err != nil {
		glog.Errorf("Failed while searching for datastore: %s. err: %v", datastorePathObj.Datastore, err)
		return nil, err
	}
	datastore := DataStore{ds}
	return datastore, nil
}

// Return Datastore object for the given datastore name
func (dc DataCenter) GetDataStoreByName(ctx context.Context, name string) (DataStore, error) {
	finder := getFinder(dc)
	ds, err := finder.Datastore(ctx, name)
	if err != nil {
		glog.Errorf("Failed while searching for datastore: %s. err %v", name, err)
		return nil, err
	}
	datastore := DataStore{ds}
	return datastore, nil
}

// Get the folder reference for the given folder name.
func (dc DataCenter) GetFolder(ctx context.Context, folderName string) (*object.Folder, error) {
	finder := getFinder(dc)
	vmFolder, err := finder.Folder(ctx, strings.TrimSuffix(folderName, "/"))
	if err != nil {
		glog.Errorf("Failed to get the folder reference for %s with err: %v", folderName, err)
		return nil, fmt.Errorf("Failed to get the folder reference for %s with err: %v", folderName, err)
	}
	return vmFolder, nil
}
