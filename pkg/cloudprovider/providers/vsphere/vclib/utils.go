package vclib

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

// Create a Dummy VM at specified location with given name.
func CreateDummyVM(ctx context.Context, datacenter DataCenter, datastore DataStore, pool *object.ResourcePool, foldername string, vmName string) (VirtualMachine, error) {
	// Create a virtual machine config spec with 1 SCSI adapter.
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{
		Name: vmName,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: "[" + datastore.Name() + "]",
		},
		NumCPUs:  1,
		MemoryMB: 4,
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationAdd,
				Device: &types.ParaVirtualSCSIController{
					VirtualSCSIController: types.VirtualSCSIController{
						SharedBus: types.VirtualSCSISharingNoSharing,
						VirtualController: types.VirtualController{
							BusNumber: 0,
							VirtualDevice: types.VirtualDevice{
								Key: 1000,
							},
						},
					},
				},
			},
		},
	}
	// Get the folder reference for global working directory where the dummy VM needs to be created.
	vmFolder, err := datacenter.GetFolder(ctx, foldername)
	if err != nil {
		glog.Errorf("Failed to get the folder reference for %s with err: %v", foldername, err)
		return nil, fmt.Errorf("Failed to get the folder reference for %q with err: %+v", foldername, err)
	}

	task, err := vmFolder.CreateVM(ctx, virtualMachineConfigSpec, pool, nil)
	if err != nil {
		glog.Errorf("Failed to create VM. err: %v", err)
		return nil, err
	}

	dummyVMTaskInfo, err := task.WaitForResult(ctx, nil)
	if err != nil {
		glog.Errorf("Error occurred while waiting for create VM task result. err: %v", err)
		return nil, err
	}

	vmRef := dummyVMTaskInfo.Result.(object.Reference)
	dummyVM := object.NewVirtualMachine(datacenter.Client(), vmRef.Reference())
	return VirtualMachine{dummyVM}, nil
}

func getFinder(dc DataCenter) *find.Finder {
	finder := find.NewFinder(dc.Client(), true)
	finder.SetDatacenter(dc.Datacenter)
	return finder
}

func CreateDiskSpec(ctx context.Context, vm VirtualMachine, datastoreMoRef types.ManagedObjectReference, diskPath string, diskControllerType string, volumeOptions VolumeOptions) (disk *types.VirtualDisk, newSCSIController types.BaseVirtualDevice, err error) {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to retrieve VM devices. err: %v", err)
		return
	}
	// find SCSI controller of particular type from VM devices
	scsiControllersOfRequiredType := getSCSIControllersOfType(vmDevices, diskControllerType)
	scsiController := getAvailableSCSIController(scsiControllersOfRequiredType)
	if scsiController == nil {
		newSCSIController, err = vm.CreateAndAttachSCSIController(ctx, diskControllerType)
		if err != nil {
			glog.Errorf("Failed to create SCSI controller for VM :%q with err: %+v", vm.Name(), err)
			return
		}

		// Get VM device list
		vmDevices, err := vm.Device(ctx)
		if err != nil {
			glog.Errorf("Failed to retrieve VM devices. err: %v", err)
			return
		}

		// verify scsi controller in virtual machine
		scsiControllersOfRequiredType := getSCSIControllersOfType(vmDevices, diskControllerType)
		scsiController := getAvailableSCSIController(scsiControllersOfRequiredType)
		if scsiController == nil {
			glog.Errorf("cannot find SCSI controller in VM")
			// attempt clean up of scsi controller
			vm.DeleteController(ctx, newSCSIController)
			err = fmt.Errorf("cannot find SCSI controller in VM")
			return
		}
	}
	disk = vmDevices.CreateDisk(scsiController, datastoreMoRef, diskPath)
	unitNumber, err := getNextUnitNumber(vmDevices, scsiController)
	if err != nil {
		glog.Errorf("cannot attach disk to VM, limit reached - %v.", err)
		return
	}
	*disk.UnitNumber = unitNumber
	backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
	backing.DiskMode = string(types.VirtualDiskModeIndependent_persistent)

	if volumeOptions.CapacityKB != 0 {
		disk.CapacityInKB = int64(volumeOptions.CapacityKB)
	}
	disk.CapacityInKB = int64(volumeOptions.CapacityKB)
	if volumeOptions.DiskFormat != "" {
		var diskFormat string
		diskFormat = diskFormatValidType[volumeOptions.DiskFormat]
		switch diskFormat {
		case ThinDiskType:
			backing.ThinProvisioned = types.NewBool(true)
		case EagerZeroedThickDiskType:
			backing.EagerlyScrub = types.NewBool(true)
		default:
			backing.ThinProvisioned = types.NewBool(false)
		}
	}
	return
}

// Filter Specific type of Controller device from given list of Virtual Machine Devices
func getSCSIControllersOfType(vmDevices object.VirtualDeviceList, scsiType string) []*types.VirtualController {
	// get virtual scsi controllers of passed argument type
	var scsiControllers []*types.VirtualController
	for _, device := range vmDevices {
		devType := vmDevices.Type(device)
		if devType == scsiType {
			if c, ok := device.(types.BaseVirtualController); ok {
				scsiControllers = append(scsiControllers, c.GetVirtualController())
			}
		}
	}
	return scsiControllers
}

// Filter and return list of Controller Devices from given list of Virtual Machine Devices.
func getSCSIControllers(vmDevices object.VirtualDeviceList) []*types.VirtualController {
	// get all virtual scsi controllers
	var scsiControllers []*types.VirtualController
	for _, device := range vmDevices {
		devType := vmDevices.Type(device)
		switch devType {
		case SCSIControllerType, strings.ToLower(LSILogicControllerType), strings.ToLower(BusLogicControllerType), PVSCSIControllerType, strings.ToLower(LSILogicSASControllerType):
			if c, ok := device.(types.BaseVirtualController); ok {
				scsiControllers = append(scsiControllers, c.GetVirtualController())
			}
		}
	}
	return scsiControllers
}

// Return available SCSI Controller from list of given controllers, which has less than 15 disk devices.
func getAvailableSCSIController(scsiControllers []*types.VirtualController) *types.VirtualController {
	// get SCSI controller which has space for adding more devices
	for _, controller := range scsiControllers {
		if len(controller.Device) < SCSIControllerDeviceLimit {
			return controller
		}
	}
	return nil
}

// Return formatted  VirtualDisk UUID
func formatVirtualDiskUUID(uuid string) string {
	uuidwithNoSpace := strings.Replace(uuid, " ", "", -1)
	uuidWithNoHypens := strings.Replace(uuidwithNoSpace, "-", "", -1)
	return strings.ToLower(uuidWithNoHypens)
}

// Return next available SCSI controller unit number from given list of Controller Device List
func getNextUnitNumber(devices object.VirtualDeviceList, c types.BaseVirtualController) (int32, error) {
	var takenUnitNumbers [SCSIDeviceSlots]bool
	takenUnitNumbers[SCSIReservedSlot] = true
	key := c.GetVirtualController().Key

	for _, device := range devices {
		d := device.GetVirtualDevice()
		if d.ControllerKey == key {
			if d.UnitNumber != nil {
				takenUnitNumbers[*d.UnitNumber] = true
			}
		}
	}
	for unitNumber, takenUnitNumber := range takenUnitNumbers {
		if !takenUnitNumber {
			return int32(unitNumber), nil
		}
	}
	return -1, fmt.Errorf("SCSI Controller with key=%d does not have any available slots.", key)
}

// Get the best fit compatible datastore by free space.
func GetMostFreeDatastore(dsMo []mo.Datastore) mo.Datastore {
	var curMax int64
	curMax = -1
	var index int
	for i, ds := range dsMo {
		dsFreeSpace := ds.Info.GetDatastoreInfo().FreeSpace
		if dsFreeSpace > curMax {
			curMax = dsFreeSpace
			index = i
		}
	}
	return dsMo[index]
}

// Get the VM list inside a folder.
func GetVMsInsideFolder(ctx context.Context, vmFolder *object.Folder, client *govmomi.Client, properties []string) ([]mo.VirtualMachine, error) {
	vmFolders, err := vmFolder.Children(ctx)
	if err != nil {
		return nil, err
	}

	pc := property.DefaultCollector(client.Client)
	var vmRefs []types.ManagedObjectReference
	var vmMoList []mo.VirtualMachine
	for _, vmFolder := range vmFolders {
		if vmFolder.Reference().Type == VirtualMachineType {
			vmRefs = append(vmRefs, vmFolder.Reference())
		}
	}
	err = pc.Retrieve(ctx, vmRefs, properties, &vmMoList)
	if err != nil {
		return nil, err
	}
	return vmMoList, nil
}
