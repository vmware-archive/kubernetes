package vclib

import (
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
)

// VolumeOptions specifies various options for a volume.
type VolumeOptions struct {
	CapacityKB             int
	Tags                   map[string]string
	Name                   string
	DiskFormat             string
	Datastore              string
	VSANStorageProfileData string
	StoragePolicyName      string
	StoragePolicyID        string
	SCSIControllerType     string
}

var (
	DiskFormatValidType = map[string]string{
		ThinDiskType:                              ThinDiskType,
		strings.ToLower(EagerZeroedThickDiskType): EagerZeroedThickDiskType,
		strings.ToLower(ZeroedThickDiskType):      PreallocatedDiskType,
	}
	// SCSIControllerValidType specifies the supported SCSI controllers
	SCSIControllerValidType = []string{LSILogicControllerType, LSILogicSASControllerType, PVSCSIControllerType}
)

// DiskformatValidOptions generates Valid Options for Diskformat
func DiskformatValidOptions() string {
	validopts := ""
	for diskformat := range DiskFormatValidType {
		validopts += diskformat + ", "
	}
	validopts = strings.TrimSuffix(validopts, ", ")
	return validopts
}

// CheckDiskFormatSupported checks if the diskFormat is valid
func CheckDiskFormatSupported(diskFormat string) bool {
	if DiskFormatValidType[diskFormat] == "" {
		glog.Errorf("Not a valid Disk Format. Valid options are %+q", DiskformatValidOptions())
		return false
	}
	return true
}

// SCSIControllerTypeValidOptions generates valid options for SCSIControllerType
func SCSIControllerTypeValidOptions() string {
	validopts := ""
	for _, controllerType := range SCSIControllerValidType {
		validopts += (controllerType + ", ")
	}
	validopts = strings.TrimSuffix(validopts, ", ")
	return validopts
}

// CheckControllerSupported checks if the given controller type is valid
func CheckControllerSupported(ctrlType string) bool {
	for _, c := range SCSIControllerValidType {
		if ctrlType == c {
			return true
		}
	}
	glog.Errorf("Not a valid SCSI Controller Type. Valid options are %q", SCSIControllerTypeValidOptions())
	return false
}

// VerifyVolumeOptions checks if volumeOptions.SCIControllerType is valid controller type
func (volumeOptions VolumeOptions) VerifyVolumeOptions() bool {
	// Validate only if SCSIControllerType is set by user.
	// Default value is set later in virtualDiskManager.Create and vmDiskManager.Create
	if volumeOptions.SCSIControllerType != "" {
		isValid := CheckControllerSupported(volumeOptions.SCSIControllerType)
		if !isValid {
			return false
		}
	}
	// ThinDiskType is the default, so skip the validation.
	if volumeOptions.DiskFormat != ThinDiskType {
		isValid := CheckDiskFormatSupported(volumeOptions.DiskFormat)
		if !isValid {
			return false
		}
	}
	return true
}
