package vclib

import (
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"golang.org/x/net/context"
)

// Folder extends the govmomi Folder object
type Folder struct {
	*object.Folder
	Datacenter *Datacenter
}

// GetVirtualMachines returns list of VirtualMachine inside a folder.
func (folder *Folder) GetVirtualMachines(ctx context.Context) ([]*VirtualMachine, error) {
	vmFolders, err := folder.Children(ctx)
	if err != nil {
		glog.Errorf("Failed to get children from Folder: %s. err: %+v", folder.InventoryPath, err)
		return nil, err
	}
	var vmObjList []*VirtualMachine
	for _, vmFolder := range vmFolders {
		if vmFolder.Reference().Type == VirtualMachineType {
			vmObj := VirtualMachine{object.NewVirtualMachine(folder.Client(), vmFolder.Reference()), folder.Datacenter}
			vmObjList = append(vmObjList, &vmObj)
		}
	}
	return vmObjList, nil
}
