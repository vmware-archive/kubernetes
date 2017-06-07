package vclib

import (
	"github.com/golang/glog"
	"strings"
)

// VolumeOptions specifies capacity, tags, name and diskFormat for a volume.
type VolumeOptions struct {
	CapacityKB             int
	Tags                   map[string]string
	Name                   string
	DiskFormat             string
	Datastore              string
	VSANStorageProfileData string
	StoragePolicyName      string
	StoragePolicyID        string
	SSCIControllerType     string
}

var (
	diskFormatValidType = map[string]string{
		ThinDiskType:                              ThinDiskType,
		strings.ToLower(EagerZeroedThickDiskType): EagerZeroedThickDiskType,
		strings.ToLower(ZeroedThickDiskType):      PreallocatedDiskType,
	}
	SCSIControllerValidType = []string{LSILogicSASControllerType, PVSCSIControllerType}
)

// Generates Valid Options for Diskformat
func DiskformatValidOptions() string {
	validopts := ""
	for diskformat := range diskFormatValidType {
		validopts += diskformat + ", "
	}
	validopts = strings.TrimSuffix(validopts, ", ")
	return validopts
}

// check given diskFormat is valid diskFormat
func CheckDiskFormatSupported(diskFormat string) bool {
	if diskFormatValidType[diskFormat] == "" {
		glog.Error("Not a valid Disk Format, Valid options are %s.", DiskformatValidOptions)
		return false
	}
	return true
}

// Generates Valid Options for SCSIControllerType
func SCSIControllerTypeValidOptions() string {
	validopts := ""
	for _, controllerType := range SCSIControllerValidType {
		validopts += (controllerType + ", ")
	}
	validopts = strings.TrimSuffix(validopts, ", ")
	return validopts
}

// check if the given controller type is valid
func CheckControllerSupported(ctrlType string) bool {
	for _, c := range SCSIControllerValidType {
		if ctrlType == c {
			return true
		}
	}
	glog.Error("Not a valid SCSI Controller Type, Valid options are %s.", SCSIControllerTypeValidOptions)
	return false
}

// check volumeOptions.SSCIControllerType is valid controller type
func (volumeOptions VolumeOptions) VerifyVolumeOptions() (valid bool) {
	valid = CheckControllerSupported(volumeOptions.SSCIControllerType)
	if !valid {
		return
	}
	valid = CheckDiskFormatSupported(volumeOptions.DiskFormat)
	return
}
