package vclib

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/pbm"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"

	pbmtypes "github.com/vmware/govmomi/pbm/types"
)

type PbmClient struct {
	*pbm.Client
	connection VSphereConnection
}

func (pbmClient *PbmClient) NewPbmClient(ctx context.Context, connection VSphereConnection) error {
	connection.Connect()
	client, err := pbm.NewClient(ctx, connection.GoVmomiClient.Client)
	if err != nil {
		glog.Errorf("Failed to create new Pbm Client. err: %v", err)
		return err
	}
	pbmClient.Client = client
	pbmClient.connection = connection
	return nil
}

// Get placement compatibility result based on storage policy requirements.
func (pbmClient *PbmClient) GetPlacementCompatibilityResult(ctx context.Context, storagePolicyID string, datastores []types.ManagedObjectReference) (pbm.PlacementCompatibilityResult, error) {
	var hubs []pbmtypes.PbmPlacementHub
	for _, ds := range datastores {
		hubs = append(hubs, pbmtypes.PbmPlacementHub{
			HubType: ds.Type,
			HubId:   ds.Value,
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
		glog.Errorf("Error occurred for CheckRequirements call. err %v", err)
		return nil, err
	}
	return res, nil
}

func (pbmClient *PbmClient) GetCompatibleDatastoresMo(ctx context.Context, compatibilityResult pbm.PlacementCompatibilityResult) ([]mo.Datastore, error) {
	compatibleHubs := compatibilityResult.CompatibleDatastores()
	// Return an error if there are no compatible datastores.
	if len(compatibleHubs) < 1 {
		glog.Errorf("There are no compatible datastores that satisfy the storage policy requirements")
		return nil, fmt.Errorf("There are no compatible datastores that satisfy the storage policy requirements")
	}
	dsMoList, err := getDatastoreMo(ctx, pbmClient.connection.GoVmomiClient, compatibleHubs)
	if err != nil {
		glog.Errorf("Failed to get datastore managed objects for compatible hubs. err %v", err)
		return nil, err
	}
	return dsMoList, nil
}

func (pbmClient *PbmClient) GetNonCompatibleDatastoresMo(ctx context.Context, compatibilityResult pbm.PlacementCompatibilityResult) []mo.Datastore {
	nonCompatibleHubs := compatibilityResult.NonCompatibleDatastores()
	// Return an error if there are no compatible datastores.
	if len(nonCompatibleHubs) < 1 {
		return nil
	}
	dsMoList, err := getDatastoreMo(ctx, pbmClient.connection.GoVmomiClient, nonCompatibleHubs)
	if err != nil {
		glog.Errorf("Failed to get datastore managed objects for non-compatible hubs. err %v", err)
		return nil
	}
	return dsMoList
}

// Get the datastore managed objects for the placement hubs using property collector.
func getDatastoreMo(ctx context.Context, govmomiClient *govmomi.Client, hubs []pbmtypes.PbmPlacementHub) ([]mo.Datastore, error) {
	var dsMoRefs []types.ManagedObjectReference
	for _, hub := range hubs {
		dsMoRefs = append(dsMoRefs, types.ManagedObjectReference{
			Type:  hub.HubType,
			Value: hub.HubId,
		})
	}
	pc := property.DefaultCollector(govmomiClient.Client)
	var dsMoList []mo.Datastore
	err := pc.Retrieve(ctx, dsMoRefs, []string{DatastoreInfoProperty}, &dsMoList)
	if err != nil {
		glog.Errorf("Failed to get datastore managed objects for placement hubs. err: %v", err)
		return nil, err
	}
	return dsMoList, nil
}
