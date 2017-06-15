package vclib

import (
	"strings"

	"github.com/golang/glog"
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
	diskFormatValidType = map[string]string{
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
	for diskformat := range diskFormatValidType {
		validopts += diskformat + ", "
	}
	validopts = strings.TrimSuffix(validopts, ", ")
	return validopts
}

// CheckDiskFormatSupported checks if the diskFormat is valid
func CheckDiskFormatSupported(diskFormat string) bool {
	if diskFormatValidType[diskFormat] == "" {
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
	valid := CheckControllerSupported(volumeOptions.SCSIControllerType)
	if !valid {
		return valid
	}
	valid = CheckDiskFormatSupported(volumeOptions.DiskFormat)
	return valid
}
