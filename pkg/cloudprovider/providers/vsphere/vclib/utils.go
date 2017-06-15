package vclib

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func getFinder(dc *Datacenter) *find.Finder {
	finder := find.NewFinder(dc.Client(), true)
	finder.SetDatacenter(dc.Datacenter)
	return finder
}

func formatVirtualDiskUUID(uuid string) string {
	uuidwithNoSpace := strings.Replace(uuid, " ", "", -1)
	uuidWithNoHypens := strings.Replace(uuidwithNoSpace, "-", "", -1)
	return strings.ToLower(uuidWithNoHypens)
}

func createDiskSpec(ctx context.Context, diskPath string, vm *VirtualMachine, dsObj *Datastore, volumeOptions VolumeOptions) (*types.VirtualDisk, types.BaseVirtualDevice, error) {
	var newSCSIController types.BaseVirtualDevice
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to retrieve VM devices. err: %+v", err)
		return nil, nil, err
	}
	// find SCSI controller of particular type from VM devices
	scsiControllersOfRequiredType := getSCSIControllersOfType(vmDevices, volumeOptions.SCSIControllerType)
	scsiController := getAvailableSCSIController(scsiControllersOfRequiredType)
	if scsiController == nil {
		newSCSIController, err = vm.createAndAttachSCSIController(ctx, volumeOptions.SCSIControllerType)
		if err != nil {
			glog.Errorf("Failed to create SCSI controller for VM :%q with err: %+v", vm.Name(), err)
			return nil, nil, err
		}
		// Get VM device list
		vmDevices, err := vm.Device(ctx)
		if err != nil {
			glog.Errorf("Failed to retrieve VM devices. err: %v", err)
			return nil, nil, err
		}
		// verify scsi controller in virtual machine
		scsiControllersOfRequiredType := getSCSIControllersOfType(vmDevices, volumeOptions.SCSIControllerType)
		scsiController = getAvailableSCSIController(scsiControllersOfRequiredType)
		if scsiController == nil {
			glog.Errorf("Cannot find SCSI controller of type: %q in VM", volumeOptions.SCSIControllerType)
			// attempt clean up of scsi controller
			vm.deleteController(ctx, newSCSIController, vmDevices)
			return nil, nil, fmt.Errorf("Cannot find SCSI controller of type: %q in VM", volumeOptions.SCSIControllerType)
		}
	}
	disk := vmDevices.CreateDisk(scsiController, dsObj.Reference(), diskPath)
	unitNumber, err := getNextUnitNumber(vmDevices, scsiController)
	if err != nil {
		glog.Errorf("Cannot attach disk to VM, unitNumber limit reached - %+v.", err)
		return nil, nil, err
	}
	*disk.UnitNumber = unitNumber
	backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
	backing.DiskMode = string(types.VirtualDiskModeIndependent_persistent)

	if volumeOptions.CapacityKB != 0 {
		disk.CapacityInKB = int64(volumeOptions.CapacityKB)
	}
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
	return disk, newSCSIController, nil
}

// getSCSIControllersOfType filters specific type of Controller device from given list of Virtual Machine Devices
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

// getAvailableSCSIController gets available SCSI Controller from list of given controllers, which has less than 15 disk devices.
func getAvailableSCSIController(scsiControllers []*types.VirtualController) *types.VirtualController {
	// get SCSI controller which has space for adding more devices
	for _, controller := range scsiControllers {
		if len(controller.Device) < SCSIControllerDeviceLimit {
			return controller
		}
	}
	return nil
}

// getNextUnitNumber gets the next available SCSI controller unit number from given list of Controller Device List
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
	return -1, fmt.Errorf("SCSI Controller with key=%d does not have any available slots", key)
}

// getSCSIControllers filters and return list of Controller Devices from given list of Virtual Machine Devices.
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
