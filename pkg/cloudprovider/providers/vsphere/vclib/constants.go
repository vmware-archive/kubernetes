package vclib

const (
	LOG_LEVEL = 4
)
const (
	RoundTripperDefaultCount = 3
	VSANDatastoreType        = "vsan"
)

// Volume Constnts
const (
	ThinDiskType             = "thin"
	PreallocatedDiskType     = "preallocated"
	EagerZeroedThickDiskType = "eagerZeroedThick"
	ZeroedThickDiskType      = "zeroedThick"
)

// Controller Constants
const (
	SCSIControllerLimit       = 4
	SCSIControllerDeviceLimit = 15
	SCSIDeviceSlots           = 16
	SCSIReservedSlot          = 7

	SCSIControllerType        = "scsi"
	LSILogicControllerType    = "lsiLogic"
	BusLogicControllerType    = "busLogic"
	LSILogicSASControllerType = "lsiLogic-sas"
	PVSCSIControllerType      = "pvscsi"
)

const (
	DatastoreProperty     = "datastore"
	DatastoreInfoProperty = "info"
	VirtualMachineType    = "VirtualMachine"
	_
)
