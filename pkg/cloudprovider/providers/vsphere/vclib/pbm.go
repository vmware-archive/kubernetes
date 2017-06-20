package vclib

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/pbm"
	"golang.org/x/net/context"

	pbmtypes "github.com/vmware/govmomi/pbm/types"
	"github.com/vmware/govmomi/vim25"
)

// PbmClient is extending govmomi pbm, and provides functions to get compatible list of datastore for given policy
type PbmClient struct {
	*pbm.Client
}

// NewPbmClient returns a new PBM Client object
func NewPbmClient(ctx context.Context, client *vim25.Client) (*PbmClient, error) {
	pbmClient, err := pbm.NewClient(ctx, client)
	if err != nil {
		glog.Errorf("Failed to create new Pbm Client. err: %+v", err)
		return nil, err
	}
	return &PbmClient{pbmClient}, nil
}

// GetCompatibleDatastores filters and returns compatible list of datastores for given storage policy id
func (pbmClient *PbmClient) GetCompatibleDatastores(ctx context.Context, storagePolicyID string, datastores []*Datastore) ([]*Datastore, error) {
	var compatibilityResult pbm.PlacementCompatibilityResult
	compatibilityResult, err := pbmClient.getPlacementCompatibilityResult(ctx, storagePolicyID, datastores)
	if err != nil {
		glog.Errorf("Error occurred while getting Placement Compatibility Result. err %+v", err)
		return nil, err
	}
	compatibleHubs := compatibilityResult.CompatibleDatastores()
	// Return an error if there are no compatible datastores.
	if len(compatibleHubs) < 1 {
		glog.Errorf("There are no compatible datastores that satisfy the storage policy requirements")
		return nil, fmt.Errorf("There are no compatible datastores that satisfy the storage policy requirements")
	}

	var compatibleDataStoreList []*Datastore
	for _, hub := range compatibleHubs {
		compatibleDataStoreList = append(compatibleDataStoreList, getDataStoreForPlacementHub(datastores, hub))
	}
	return compatibleDataStoreList, nil
}

// get placement compatibility result based on storage policy requirements.
func (pbmClient *PbmClient) getPlacementCompatibilityResult(ctx context.Context, storagePolicyID string, datastore []*Datastore) (pbm.PlacementCompatibilityResult, error) {
	var hubs []pbmtypes.PbmPlacementHub
	for _, ds := range datastore {
		hubs = append(hubs, pbmtypes.PbmPlacementHub{
			HubType: ds.Reference().Type,
			HubId:   ds.Reference().Value,
		})
	}
	req := []pbmtypes.BasePbmPlacementRequirement{
		&pbmtypes.PbmPlacementCapabilityProfileRequirement{
			ProfileId: pbmtypes.PbmProfileId{
				UniqueId: storagePolicyID,
			},
		},
	}
	res, err := pbmClient.CheckRequirements(ctx, hubs, nil, req)
	if err != nil {
		glog.Errorf("Error occurred for CheckRequirements call. err %+v", err)
		return nil, err
	}
	return res, nil
}

// getDataStoreForPlacementHub returns matching datastore associated with given pbmPlacementHub
func getDataStoreForPlacementHub(datastore []*Datastore, pbmPlacementHub pbmtypes.PbmPlacementHub) *Datastore {
	for _, ds := range datastore {
		if ds.Reference().Type == pbmPlacementHub.HubType && ds.Reference().Value == pbmPlacementHub.HubId {
			return ds
		}
	}
	return nil
}
