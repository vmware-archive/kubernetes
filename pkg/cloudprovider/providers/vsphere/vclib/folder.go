package vclib

import (
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"golang.org/x/net/context"
)

// Folder extends the govmomi Folder object
type Folder struct {
	*object.Folder
	FolderPath string
	Datacenter *Datacenter
}

// GetVirtualMachines returns list of VirtualMachine inside a folder.
func (folder *Folder) GetVirtualMachines(ctx context.Context) ([]*VirtualMachine, error) {
	virtualMachines, err := getFinder(folder.Datacenter).VirtualMachineList(ctx, folder.FolderPath)
	if err != nil {
		glog.Errorf("Failed to get VirtualMachineList from Folder. err: %+v", err)
		return nil, err
	}
	var vmObjectList []*VirtualMachine
	for _, vm := range virtualMachines {
		vmObjct := VirtualMachine{vm, folder.Datacenter}
		vmObjectList = append(vmObjectList, &vmObjct)
	}
	return vmObjectList, nil
}
