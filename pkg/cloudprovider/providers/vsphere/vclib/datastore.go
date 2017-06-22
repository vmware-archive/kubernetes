package vclib

import (
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

// Datastore extends the govmomi Datastore object
type Datastore struct {
	*object.Datastore
	Datacenter *Datacenter
}

// CreateDirectory creates the directory at location specified by directoryPath.
// If the intermediate level folders do not exist, and the parameter createParents is true, all the non-existent folders are created.
// directoryPath must be in the format "[vsanDatastore] kubevols"
func (ds *Datastore) CreateDirectory(ctx context.Context, directoryPath string, createParents bool) error {
	fileManager := object.NewFileManager(ds.Client())
	err := fileManager.MakeDirectory(ctx, directoryPath, ds.Datacenter.Datacenter, createParents)
	if err != nil {
		if soap.IsSoapFault(err) {
			soapFault := soap.ToSoapFault(err)
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); ok {
				glog.Warning("Cannot create dir: %s. err: %v", directoryPath, ErrFileAlreadyExist)
				return ErrFileAlreadyExist
			}
		}
		return err
	}
	glog.V(LogLevel).Infof("Created dir with path as %+q", directoryPath)
	return nil
}

// GetType returns the type of datastore
func (ds *Datastore) GetType(ctx context.Context) (string, error) {
	var dsMo mo.Datastore
	pc := property.DefaultCollector(ds.Client())
	err := pc.RetrieveOne(ctx, ds.Datastore.Reference(), []string{"summary"}, &dsMo)
	if err != nil {
		glog.Errorf("Failed to retrieve datastore summary property. err: %v", err)
		return "", err
	}
	return dsMo.Summary.Type, nil
}

// IsCompatibleWithStoragePolicy returns true if datastore is compatible with given storage policy else return false
// for not compatible datastore, fault message is also returned
func (ds *Datastore) IsCompatibleWithStoragePolicy(ctx context.Context, storagePolicyID string) (bool, string, error) {
	pbmClient, err := NewPbmClient(ctx, ds.Client())
	if err != nil {
		glog.Errorf("Failed to get new PbmClient Object. err: %v", err)
		return false, "", err
	}
	return pbmClient.IsDatastoreCompatible(ctx, storagePolicyID, ds)
}
