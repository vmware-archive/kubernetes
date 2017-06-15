package vclib

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VirtualMachine extends the govmomi VirtualMachine object
type VirtualMachine struct {
	*object.VirtualMachine
	datacenter *Datacenter
}

// IsDiskAttached checks if disk is attached to the VM.
func (vm *VirtualMachine) IsDiskAttached(ctx context.Context, diskPath string) (bool, error) {
	// Get object key of controller to which disk is attached
	_, err := vm.GetVirtualDiskControllerKey(ctx, diskPath)
	if err != nil {
		if err == ErrNoDevicesFound {
			return false, nil
		}
		glog.Errorf("Failed to check whether disk is attached to VM: %q. err: %s", vm.Name(), err)
		return false, err
	}
	return true, nil
}

// GetVirtualDiskUUIDByPath gets the virtual disk UUID by diskPath
func (vm *VirtualMachine) GetVirtualDiskUUIDByPath(ctx context.Context, diskPath string) (string, error) {
	if len(diskPath) > 0 && filepath.Ext(diskPath) != ".vmdk" {
		diskPath += ".vmdk"
	}
	vdm := object.NewVirtualDiskManager(vm.Client())
	// Returns uuid of vmdk virtual disk
	diskUUID, err := vdm.QueryVirtualDiskUuid(ctx, diskPath, vm.datacenter.Datacenter)

	if err != nil {
		glog.Errorf("QueryVirtualDiskUuid failed for diskPath: %q on VM: %q. err: %+v", diskPath, vm.Name(), err)
		return "", ErrNoDiskUUIDFound
	}
	diskUUID = formatVirtualDiskUUID(diskUUID)
	return diskUUID, nil
}

// GetVirtualDiskControllerKey gets the object key that denotes the controller object to which vmdk is attached.
func (vm *VirtualMachine) GetVirtualDiskControllerKey(ctx context.Context, diskPath string) (int32, error) {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to get the devices for vm: %q. err: %+v", vm.Name(), err)
		return -1, err
	}
	device, err := vm.getVirtualDeviceByPath(ctx, vmDevices, diskPath)
	if err != nil {
		glog.Errorf("Failed to get virtualDevice for path: %q on VM: %q. err: %+v", diskPath, vm.Name(), err)
		return -1, err
	} else if device != nil {
		return device.GetVirtualDevice().ControllerKey, nil
	}
	return -1, ErrNoDevicesFound
}

// DeleteVM deletes the VM.
func (vm *VirtualMachine) DeleteVM(ctx context.Context) error {
	destroyTask, err := vm.Destroy(ctx)
	if err != nil {
		glog.Errorf("Failed to delete the VM: %q. err: %+v", vm.Name(), err)
		return err
	}
	return destroyTask.Wait(ctx)
}

// AttachDisk attaches the disk at location - vmDiskPath from Datastore - dsObj to the Virtual Machine
// Additionally the disk can be configured with SPBM policy if volumeOptions.StoragePolicyID is non-empty.
func (vm *VirtualMachine) AttachDisk(ctx context.Context, vmDiskPath string, volumeOptions VolumeOptions) (string, error) {
	// Check if the diskControllerType is valid
	if !CheckControllerSupported(volumeOptions.SCSIControllerType) {
		return "", fmt.Errorf("Not a valid SCSI Controller Type. Valid options are %q", SCSIControllerTypeValidOptions())
	}
	attached, err := vm.IsDiskAttached(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("Error occurred while checking if disk is attached on VM: %q. vmDiskPath: %q, err: %+v", vm.Name(), vmDiskPath, err)
		return "", err
	}
	// If disk is already attached, return the disk UUID
	if attached {
		diskUUID, _ := vm.GetVirtualDiskUUIDByPath(ctx, vmDiskPath)
		return diskUUID, nil
	}

	dsObj, err := vm.datacenter.GetDatastoreByPath(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("Failed to get datastore from vmDiskPath: %q. err: %+v", vmDiskPath, err)
		return "", err
	}
	// If disk is not attached, create a disk spec for disk to be attached to the VM.
	disk, newSCSIController, err := createDiskSpec(ctx, vmDiskPath, vm, dsObj, volumeOptions)
	if err != nil {
		glog.Errorf("Error occurred while creating disk spec. err: %+v", err)
		return "", err
	}
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to retrieve VM devices for VM: %q. err: %+v", vm.Name(), err)
		return "", err
	}
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{}
	deviceConfigSpec := &types.VirtualDeviceConfigSpec{
		Device:    disk,
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
	}
	// Configure the disk with the SPBM profile only if ProfileID is not empty.
	if volumeOptions.StoragePolicyID != "" {
		profileSpec := &types.VirtualMachineDefinedProfileSpec{
			ProfileId: volumeOptions.StoragePolicyID,
		}
		deviceConfigSpec.Profile = append(deviceConfigSpec.Profile, profileSpec)
	}
	virtualMachineConfigSpec.DeviceChange = append(virtualMachineConfigSpec.DeviceChange, deviceConfigSpec)
	task, err := vm.Reconfigure(ctx, virtualMachineConfigSpec)
	if err != nil {
		glog.Errorf("Failed to attach the disk with storagePolicy: %q on VM: %q. err - %+v", volumeOptions.StoragePolicyID, vm.Name(), err)
		if newSCSIController != nil {
			vm.deleteController(ctx, newSCSIController, vmDevices)
		}
		return "", err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to attach the disk with storagePolicy: %+q on VM: %q. err - %+v", volumeOptions.StoragePolicyID, vm.Name(), err)
		if newSCSIController != nil {
			vm.deleteController(ctx, newSCSIController, vmDevices)
		}
		return "", err
	}

	// Once disk is attached, get the disk UUID.
	diskUUID, err := getVirtualDiskUUIDByDisk(ctx, vmDevices, disk)
	if err != nil {
		glog.Errorf("Error occurred while getting Disk Info from VM: %q. err: %v", vm.Name(), err)
		if newSCSIController != nil {
			vm.deleteController(ctx, newSCSIController, vmDevices)
		}
		vm.DetachDisk(ctx, vmDiskPath)
		return "", err
	}
	return diskUUID, nil
}

// DetachDisk detaches the disk specified by vmDiskPath
func (vm *VirtualMachine) DetachDisk(ctx context.Context, vmDiskPath string) error {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Error occurred while getting VM devices for VM: %q. err: %v", vm.Name(), err)
		return err
	}
	diskID, err := vm.getVirtualDiskID(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("disk ID not found for VM: %q with diskPath: %q", vm.Name(), vmDiskPath)
		return err
	}
	// Gets virtual disk device
	device := vmDevices.Find(diskID)
	if device == nil {
		glog.Errorf("device '%s' not found for VM: %q", diskID, vm.Name())
		return fmt.Errorf("device '%s' not found for VM: %q", diskID, vm.Name())
	}
	// Detach disk from VM
	err = vm.RemoveDevice(ctx, true, device)
	if err != nil {
		glog.Errorf("Error occurred while removing disk device for VM: %q. err: %v", vm.Name(), err)
		return err
	}
	return nil
}

// GetResourcePool gets the resource pool for VM.
func (vm *VirtualMachine) GetResourcePool(ctx context.Context) (*object.ResourcePool, error) {
	vmMoList, err := vm.datacenter.GetVMMoList(ctx, []*VirtualMachine{vm}, []string{"resourcePool"})
	if err != nil {
		glog.Errorf("Failed to get resource pool from VM: %q. err: %+v", vm.Name(), err)
		return nil, err
	}
	return object.NewResourcePool(vm.Client(), vmMoList[0].ResourcePool.Reference()), nil
}

// GetAllAccessibleDatastores gets the list of accessible Datastores for the given Virtual Machine
func (vm *VirtualMachine) GetAllAccessibleDatastores(ctx context.Context) ([]Datastore, error) {
	host, err := vm.HostSystem(ctx)
	if err != nil {
		glog.Errorf("Failed to get host system for VM: %q. err: %+v", vm.Name(), err)
		return nil, err
	}
	var hostSystemMo mo.HostSystem
	s := object.NewSearchIndex(vm.Client())
	err = s.Properties(ctx, host.Reference(), []string{DatastoreProperty}, &hostSystemMo)
	if err != nil {
		glog.Errorf("Failed to retrieve datastores for host: %+v. err: %+v", host, err)
		return nil, err
	}
	var dsObjList []Datastore
	for _, dsRef := range hostSystemMo.Datastore {
		dsObjList = append(dsObjList, Datastore{object.NewDatastore(vm.Client(), dsRef), vm.datacenter})
	}
	return dsObjList, nil
}

// createAndAttachSCSIController creates and attachs the SCSI controller to the VM.
func (vm *VirtualMachine) createAndAttachSCSIController(ctx context.Context, diskControllerType string) (types.BaseVirtualDevice, error) {
	// Get VM device list
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to retrieve VM devices for VM: %q. err: %+v", vm.Name(), err)
		return nil, err
	}
	allSCSIControllers := getSCSIControllers(vmDevices)
	if len(allSCSIControllers) >= SCSIControllerLimit {
		// we reached the maximum number of controllers we can attach
		glog.Errorf("SCSI Controller Limit of %d has been reached, cannot create another SCSI controller", SCSIControllerLimit)
		return nil, fmt.Errorf("SCSI Controller Limit of %d has been reached, cannot create another SCSI controller", SCSIControllerLimit)
	}
	newSCSIController, err := vmDevices.CreateSCSIController(diskControllerType)
	if err != nil {
		glog.Errorf("Failed to create new SCSI controller on VM: %q. err: %+v", vm.Name(), err)
		return nil, err
	}
	configNewSCSIController := newSCSIController.(types.BaseVirtualSCSIController).GetVirtualSCSIController()
	hotAndRemove := true
	configNewSCSIController.HotAddRemove = &hotAndRemove
	configNewSCSIController.SharedBus = types.VirtualSCSISharing(types.VirtualSCSISharingNoSharing)

	// add the scsi controller to virtual machine
	err = vm.AddDevice(context.TODO(), newSCSIController)
	if err != nil {
		glog.V(LogLevel).Infof("Cannot add SCSI controller to VM: %q. err: %+v", vm.Name(), err)
		// attempt clean up of scsi controller
		vm.deleteController(ctx, newSCSIController, vmDevices)
		return nil, err
	}
	return newSCSIController, nil
}

// getVirtualDeviceByPath gets the virtual device by path
func (vm *VirtualMachine) getVirtualDeviceByPath(ctx context.Context, vmDevices object.VirtualDeviceList, diskPath string) (types.BaseVirtualDevice, error) {
	volumeUUID, err := vm.GetVirtualDiskUUIDByPath(ctx, diskPath)
	if err != nil {
		glog.Errorf("Failed to get disk UUID for path: %q on VM: %q. err: %+v", diskPath, vm.Name(), err)
		return nil, err
	}
	// filter vm devices to retrieve device for the given vmdk file identified by disk path
	for _, device := range vmDevices {
		if vmDevices.TypeName(device) == "VirtualDisk" {
			diskUUID, _ := getVirtualDiskUUIDByDevice(device)
			if diskUUID == volumeUUID {
				return device, nil
			}
		}
	}
	return nil, nil
}

// deleteController removes latest added SCSI controller from VM.
func (vm *VirtualMachine) deleteController(ctx context.Context, controllerDevice types.BaseVirtualDevice, vmDevices object.VirtualDeviceList) error {
	controllerDeviceList := vmDevices.SelectByType(controllerDevice)
	if len(controllerDeviceList) < 1 {
		return ErrNoDevicesFound
	}
	device := controllerDeviceList[len(controllerDeviceList)-1]
	err := vm.RemoveDevice(ctx, true, device)
	if err != nil {
		glog.Errorf("Error occurred while removing device on VM: %q. err: %+v", vm.Name(), err)
		return err
	}
	return nil
}

// getVirtualDiskID gets a device ID which is internal vSphere API identifier for the attached virtual disk.
func (vm *VirtualMachine) getVirtualDiskID(ctx context.Context, diskPath string) (string, error) {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Failed to get the devices for VM: %q. err: %+v", vm.Name(), err)
		return "", err
	}
	device, err := vm.getVirtualDeviceByPath(ctx, vmDevices, diskPath)
	if err != nil {
		glog.Errorf("Failed to get virtualDevice for path: %q on VM: %q. err: %+v", diskPath, vm.Name(), err)
		return "", err
	} else if device != nil {
		return vmDevices.Name(device), nil
	}
	return "", ErrNoDiskIDFound
}

// getVirtualDiskUUIDByDisk gets a disk UUID for the virtual disk.
func getVirtualDiskUUIDByDisk(ctx context.Context, vmDevices object.VirtualDeviceList, disk *types.VirtualDisk) (string, error) {
	devices := vmDevices.SelectByType(disk)
	if len(devices) < 1 {
		return "", ErrNoDevicesFound
	}
	// get new disk id
	newDevice := devices[len(devices)-1]
	// get device uuid
	diskUUID, err := getVirtualDiskUUIDByDevice(newDevice)
	if err != nil {
		glog.Errorf("Error occurred while getting Disk UUID of the device. err: %+v", err)
		return "", err
	}

	return diskUUID, nil
}

// getVirtualDiskUUIDByDevice gets the disk UUID by device
func getVirtualDiskUUIDByDevice(newDevice types.BaseVirtualDevice) (string, error) {
	virtualDevice := newDevice.GetVirtualDevice()
	if backing, ok := virtualDevice.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
		uuid := formatVirtualDiskUUID(backing.Uuid)
		return uuid, nil
	}
	return "", ErrNoDiskUUIDFound
}
