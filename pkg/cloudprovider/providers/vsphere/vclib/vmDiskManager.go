package vclib

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

type vmDiskManager struct {
	diskPath      string
	volumeOptions VolumeOptions
	vmOptions     VMOptions
}

// Create implements Disk's Create interface
// Contains implementation of VM based Provisioning to provision disk with SPBM Policy or VSANStorageProfileData
func (vmdisk vmDiskManager) Create(ctx context.Context, datastore *Datastore) (err error) {
	storageProfileSpec := &types.VirtualMachineDefinedProfileSpec{}
	// Is PBM storage policy ID is present, set the storage spec profile ID,
	// else, set raw the VSAN policy string.
	if vmdisk.volumeOptions.StoragePolicyID != "" {
		storageProfileSpec.ProfileId = vmdisk.volumeOptions.StoragePolicyID
	} else if vmdisk.volumeOptions.VSANStorageProfileData != "" {
		// Check Datastore type - VSANStorageProfileData is only applicable to vSAN Datastore
		dsType, err := datastore.GetType(ctx)
		if err != nil {
			return err
		}
		if dsType != VSANDatastoreType {
			glog.Errorf("The specified datastore: %q is not a VSAN datastore", datastore.Name())
			return fmt.Errorf("The specified datastore: %q is not a VSAN datastore."+
				" The policy parameters will work only with VSAN Datastore."+
				" So, please specify a valid VSAN datastore in Storage class definition.", datastore.Name())
		}
		storageProfileSpec.ProfileId = ""
		storageProfileSpec.ProfileData = &types.VirtualMachineProfileRawData{
			ExtensionKey: "com.vmware.vim.sps",
			ObjectData:   vmdisk.volumeOptions.VSANStorageProfileData,
		}
	}
	var dummyVM *VirtualMachine
	// Check if VM already exist in the folder.
	// If VM is already present, use it, else create a new dummy VM.
	dummyVMFullName := DummyVMPrefixName + "-" + vmdisk.volumeOptions.Name
	dummyVM, err = datastore.datacenter.GetVMByPath(ctx, vmdisk.vmOptions.WorkingDirectoryPath+"//"+dummyVMFullName)
	if err != nil {
		// Create a dummy VM
		dummyVM, err = vmdisk.createDummyVM(ctx, datastore.datacenter, vmdisk.volumeOptions, dummyVMFullName)
		if err != nil {
			glog.Errorf("Failed to create Dummy VM. err: %v", err)
			return err
		}
	}

	// Reconfigure the VM to attach the disk with the VSAN policy configured
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{}
	disk, _, err := createDiskSpec(ctx, dummyVM, vmdisk.diskPath, datastore, vmdisk.volumeOptions)
	if err != nil {
		glog.Errorf("Failed to create Disk Spec. err: %v", err)
		return err
	}
	deviceConfigSpec := &types.VirtualDeviceConfigSpec{
		Device:        disk,
		Operation:     types.VirtualDeviceConfigSpecOperationAdd,
		FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
	}

	deviceConfigSpec.Profile = append(deviceConfigSpec.Profile, storageProfileSpec)
	virtualMachineConfigSpec.DeviceChange = append(virtualMachineConfigSpec.DeviceChange, deviceConfigSpec)
	fileAlreadyExist := false
	task, err := dummyVM.Reconfigure(ctx, virtualMachineConfigSpec)
	err = task.Wait(ctx)
	if err != nil {
		errorMessage := fmt.Sprintf("Cannot complete the operation because the file or folder %s already exists", vmdisk.diskPath)
		if errorMessage == err.Error() {
			//Skip error and continue to detach the disk as the disk was already created on the datastore.
			fileAlreadyExist = true
			glog.V(LogLevel).Info("File: %v already exists", vmdisk.diskPath)
		} else {
			glog.Errorf("Failed to attach the disk to VM: %q with err: %+v", dummyVMFullName, err)
			return err
		}
	}
	// Detach the disk from the dummy VM.
	err = dummyVM.DetachDisk(ctx, vmdisk.diskPath)
	if err != nil {
		if DiskNotFoundErrMsg == err.Error() && fileAlreadyExist {
			// Skip error if disk was already detached from the dummy VM but still present on the datastore.
			glog.V(LogLevel).Info("File: %v is already detached", vmdisk.diskPath)
		} else {
			glog.Errorf("Failed to detach the disk: %q from VM: %q with err: %+v", vmdisk.diskPath, dummyVMFullName, err)
			return err
		}
	}
	//  Delete the dummy VM
	err = dummyVM.DeleteVM(ctx)
	if err != nil {
		return fmt.Errorf("Failed to destroy the vm: %q with err: %+v", dummyVMFullName, err)
	}
	return nil
}

// CreateDummyVM create a Dummy VM at specified location with given name.
func (vmdisk vmDiskManager) createDummyVM(ctx context.Context, datacenter *Datacenter, volumeOptions VolumeOptions, vmName string) (*VirtualMachine, error) {
	// Create a virtual machine config spec with 1 SCSI adapter.
	virtualMachineConfigSpec := types.VirtualMachineConfigSpec{
		Name: vmName,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: "[" + volumeOptions.Datastore + "]",
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

	task, err := vmdisk.vmOptions.WorkingDirectoryFolder.CreateVM(ctx, virtualMachineConfigSpec, vmdisk.vmOptions.VmResourcePool, nil)
	if err != nil {
		glog.Errorf("Failed to create VM. err: %+v", err)
		return nil, err
	}

	dummyVMTaskInfo, err := task.WaitForResult(ctx, nil)
	if err != nil {
		glog.Errorf("Error occurred while waiting for create VM task result. err: %+v", err)
		return nil, err
	}

	vmRef := dummyVMTaskInfo.Result.(object.Reference)
	dummyVM := object.NewVirtualMachine(datacenter.Client(), vmRef.Reference())
	return &VirtualMachine{dummyVM, datacenter}, nil
}
