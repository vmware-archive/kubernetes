package vclib

import (
	"github.com/vmware/govmomi/object"
)

// VMOptions provides helper objects for provisioning volume with SPBM Policy
type VMOptions struct {
	WorkingDirectoryPath   string
	WorkingDirectoryFolder *Folder
	VmResourcePool         *object.ResourcePool
}
