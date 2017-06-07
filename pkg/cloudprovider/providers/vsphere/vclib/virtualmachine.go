package vclib

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
	"path/filepath"
)

type VirtualMachine struct {
	*object.VirtualMachine
}

// Check if Disk is attached to the VM, if Yes return true else return false
func (vm VirtualMachine) IsDiskAttached(ctx context.Context, diskPath string) (bool, error) {
	// Get devices from VM
	_, err := vm.GetVirtualDiskControllerKey(ctx, diskPath)
	if err != nil {
		if err == ErrNoDevicesFound {
			return false, nil
		}
		glog.Errorf("Failed to check whether disk is attached. err: %s", err)
		return false, err
	}
	return true, nil
}

// Returns the object key that denotes the controller object to which vmdk is attached.
func (vm VirtualMachine) GetVirtualDiskControllerKey(ctx context.Context, diskPath string) (int32, error) {
	volumeUUID, err := vm.GetVirtualDiskUUIDByPath(ctx, diskPath)
	if err != nil {
		glog.Errorf("disk uuid not found for %v. err: %s", diskPath, err)
		return -1, err
	}
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		return -1, err
	}
	// filter vm devices to retrieve disk ID for the given vmdk file
	for _, device := range vmDevices {
		if vmDevices.TypeName(device) == "VirtualDisk" {
			diskUUID, _ := GetVirtualDiskUUIDByDevice(device)
			if diskUUID == volumeUUID {
				return device.GetVirtualDevice().ControllerKey, nil
			}
		}
	}
	return -1, ErrNoDevicesFound
}

// Returns disk UUID for a virtual disk device.
func GetVirtualDiskUUIDByDevice(newDevice types.BaseVirtualDevice) (string, error) {
	virtualDevice := newDevice.GetVirtualDevice()
	if backing, ok := virtualDevice.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
		uuid := formatVirtualDiskUUID(backing.Uuid)
		return uuid, nil
	}
	return "", ErrNoDiskUUIDFound
}

// Return disk UUID for a virtual disk path.
func (vm VirtualMachine) GetVirtualDiskUUIDByPath(ctx context.Context, diskPath string) (string, error) {
	if len(diskPath) > 0 && filepath.Ext(diskPath) != ".vmdk" {
		diskPath += ".vmdk"
	}
	vdm := object.NewVirtualDiskManager(vm.Client())
	// Returns uuid of vmdk virtual disk
	diskUUID, err := vdm.QueryVirtualDiskUuid(ctx, diskPath, nil)

	if err != nil {
		glog.Errorf("QueryVirtualDiskUuid failed for diskPath: %s, err: %v", diskPath, err)
		return "", ErrNoDiskUUIDFound
	}
	diskUUID = formatVirtualDiskUUID(diskUUID)
	return diskUUID, nil
}

// Returns a device id which is internal vSphere API identifier for the attached virtual disk.
func (vm VirtualMachine) GetVirtualDiskID(ctx context.Context, diskPath string) (string, error) {
	volumeUUID, err := vm.GetVirtualDiskUUIDByPath(ctx, diskPath)
	if err != nil {
		glog.Errorf("disk uuid not found for %v ", diskPath)
		return "", err
	}
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		return nil, err
	}
	// filter vm devices to retrieve disk ID for the given vmdk file
	for _, device := range vmDevices {
		if vmDevices.TypeName(device) == "VirtualDisk" {
			diskUUID, _ := GetVirtualDiskUUIDByDevice(device)
			if diskUUID == volumeUUID {
				return vmDevices.Name(device), nil
			}
		}
	}
	return "", ErrNoDiskIDFound
}

// Delete the VM.
func (vm VirtualMachine) DeleteVM(ctx context.Context) error {
	destroyTask, err := vm.Destroy(ctx)
	if err != nil {
		return err
	}
	return destroyTask.Wait(ctx)
}

// Attach disk to the Virtual Machine
func (vm VirtualMachine) AttachDisk(ctx context.Context, vmDiskPath string, storagePolicyID string, diskControllerType string, dsRef types.ManagedObjectReference) (diskID string, diskUUID string, err error) {
	var newSCSIController types.BaseVirtualDevice
	attached, err := vm.IsDiskAttached(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("Error occurred while checking disk attachment: Disk Path: %s, err: %v", vmDiskPath, err)
		return "", "", err
	}
	if attached {
		diskID, _ = vm.GetVirtualDiskID(ctx, vmDiskPath)
		diskUUID, _ = vm.GetVirtualDiskUUIDByPath(ctx, vmDiskPath)
		return diskID, diskUUID, nil
	}

	disk, newSCSIController, err := CreateDiskSpec(ctx, vm, dsRef, vmDiskPath, diskControllerType, VolumeOptions{})
	if err != nil {
		glog.Errorf("Error occurred while creating disk spec, err: %v", err)
		return "", "", err
	}
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{}
	deviceConfigSpec := &types.VirtualDeviceConfigSpec{
		Device:    disk,
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
	}

	// Configure the disk with the SPBM profile only if ProfileID is not empty.
	if storagePolicyID != "" {
		profileSpec := &types.VirtualMachineDefinedProfileSpec{
			ProfileId: storagePolicyID,
		}
		deviceConfigSpec.Profile = append(deviceConfigSpec.Profile, profileSpec)
	}
	virtualMachineConfigSpec.DeviceChange = append(virtualMachineConfigSpec.DeviceChange, deviceConfigSpec)
	task, err := vm.Reconfigure(ctx, virtualMachineConfigSpec)
	if err != nil {
		glog.Errorf("Failed to attach the disk with storagePolicy: %+q with err - %v", storagePolicyID, err)
		if newSCSIController != nil {
			vm.DeleteController(ctx, newSCSIController)
		}
		return "", "", err
	}
	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Failed to attach the disk with storagePolicy: %+q with err - %v", storagePolicyID, err)
		if newSCSIController != nil {
			vm.DeleteController(ctx, newSCSIController)
		}
		return "", "", err
	}

	deviceName, diskUUID, err := vm.GetVMDiskInfo(ctx, disk)
	if err != nil {
		glog.Errorf("Error occurred while getting Disk Info, err: %v", err)
		if newSCSIController != nil {
			vm.DeleteController(ctx, newSCSIController)
		}
		vm.DetachDisk(ctx, vmDiskPath)
		return "", "", err
	}
	return deviceName, diskUUID, nil
}

func (vm VirtualMachine) GetVMDiskInfo(ctx context.Context, disk *types.VirtualDisk) (string, string, error) {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Error occurred while getting VM devices, err: %v", err)
		return "", "", err
	}
	devices := vmDevices.SelectByType(disk)
	if len(devices) < 1 {
		return "", "", ErrNoDevicesFound
	}

	// get new disk id
	newDevice := devices[len(devices)-1]
	deviceName := devices.Name(newDevice)

	// get device uuid
	diskUUID, err := GetVirtualDiskUUIDByDevice(newDevice)
	if err != nil {
		glog.Errorf("Error occurred while getting Disk UUID of the device, err: %v", err)
		return "", "", err
	}

	return deviceName, diskUUID, nil
}

// DetachDisk detaches the disk specified by vmDiskPath
func (vm VirtualMachine) DetachDisk(ctx context.Context, vmDiskPath string) error {
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Error occurred while getting VM devices, err: %v", err)
		return err
	}
	diskID, err := vm.GetVirtualDiskID(ctx, vmDiskPath)
	if err != nil {
		glog.Errorf("disk ID not found for %v ", vmDiskPath)
		return err
	}
	// Gets virtual disk device
	device := vmDevices.Find(diskID)
	if device == nil {
		glog.Errorf("device '%s' not found", diskID)
		return fmt.Errorf("device '%s' not found", diskID)
	}
	// Detach disk from VM
	err = vm.RemoveDevice(ctx, true, device)
	if err != nil {
		glog.Errorf("Error occurred while removing disk device, err: %v", err)
		return err
	}
	return nil
}

// Create and Attach a SCSI controller to VM.
func (vm VirtualMachine) CreateAndAttachSCSIController(ctx context.Context, diskControllerType string) (types.BaseVirtualDevice, error) {
	// Get VM device list
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Error occurred while getting VM devices, err: %v", err)
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
		glog.Errorf("error creating new SCSI controller: %v", err)
		return nil, err
	}
	configNewSCSIController := newSCSIController.(types.BaseVirtualSCSIController).GetVirtualSCSIController()
	hotAndRemove := true
	configNewSCSIController.HotAddRemove = &hotAndRemove
	configNewSCSIController.SharedBus = types.VirtualSCSISharing(types.VirtualSCSISharingNoSharing)

	// add the scsi controller to virtual machine
	err = vm.AddDevice(context.TODO(), newSCSIController)
	if err != nil {
		glog.V(LOG_LEVEL).Infof("cannot add SCSI controller to vm. err: %v", err)
		// attempt clean up of scsi controller
		vm.DeleteController(ctx, newSCSIController)
		return nil, err
	}
	return newSCSIController, nil
}

// Get VM's Resource Pool
func (vm VirtualMachine) GetResourcePool(ctx context.Context) (*object.ResourcePool, error) {
	currentVMHost, err := vm.HostSystem(ctx)
	if err != nil {
		glog.Errorf("Failed to get hostsystem for VM, err: %v", err)
		return nil, err
	}
	// Get the resource pool for the current node.
	// We create the dummy VM in the same resource pool as current node.
	resourcePool, err := currentVMHost.ResourcePool(ctx)
	if err != nil {
		glog.Errorf("Failed to get resource pool of the VM, err: %v", err)
		return nil, err
	}
	return resourcePool, nil
}

// Removes latest added SCSI controller from VM.
func (vm VirtualMachine) DeleteController(ctx context.Context, controllerDevice types.BaseVirtualDevice) error {
	if controllerDevice == nil {
		glog.Errorf("Nil value is set for controllerDevice")
		return fmt.Errorf("Nil value is set for controllerDevice")
	}
	if vm.VirtualMachine == nil {
		glog.Errorf("Nil value is set for vm.VirtualMachine")
		return fmt.Errorf("Nil value is set for vm.VirtualMachine")
	}
	// Get VM device list
	vmDevices, err := vm.Device(ctx)
	if err != nil {
		glog.Errorf("Error occurred while getting VM devices, err: %v", err)
		return err
	}
	controllerDeviceList := vmDevices.SelectByType(controllerDevice)
	if len(controllerDeviceList) < 1 {
		return ErrNoDevicesFound
	}
	device := controllerDeviceList[len(controllerDeviceList)-1]
	err = vm.RemoveDevice(ctx, true, device)
	if err != nil {
		glog.Errorf("Error occurred while removing device, err: %v", err)
		return err
	}
	return nil
}

// Get the list of Accessible Datastores for the given Virtual Machine
func (vm VirtualMachine) GetAllAccessibleDatastores(ctx context.Context) ([]string, error) {
	host, err := vm.HostSystem(ctx)
	if err != nil {
		return nil, err
	}
	var hostSystemMo mo.HostSystem
	s := object.NewSearchIndex(vm.Client())
	err = s.Properties(ctx, host.Reference(), []string{DatastoreProperty}, &hostSystemMo)
	if err != nil {
		return nil, err
	}
	var dsRefValues []string
	for _, dsRef := range hostSystemMo.Datastore {
		dsRefValues = append(dsRefValues, dsRef.Value)
	}
	return dsRefValues, nil
}
