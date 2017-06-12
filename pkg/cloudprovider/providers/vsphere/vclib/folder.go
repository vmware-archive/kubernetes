package vclib

import (
	"github.com/vmware/govmomi/object"
)

// Folder extends the govmomi Folder object
type Folder struct {
	*object.Folder
}
