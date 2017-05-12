/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vsphere

import (
	"context"
	"os"
	"runtime"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"fmt"

	pbmtypes "github.com/vmware/govmomi/pbm/types"
)

const (
	ClusterComputeResource = "ClusterComputeResource"
	ParentProperty         = "parent"
	DatastoreProperty      = "datastore"
	DatastoreInfoProperty  = "info"
)

// Reads vSphere configuration from system environment and construct vSphere object
func GetVSphere() (*VSphere, error) {
	cfg := getVSphereConfig()
	client, err := GetgovmomiClient(cfg)
	if err != nil {
		return nil, err
	}
	vs := &VSphere{
		client:          client,
		cfg:             cfg,
		localInstanceID: "",
	}
	runtime.SetFinalizer(vs, logout)
	return vs, nil
}

func getVSphereConfig() *VSphereConfig {
	var cfg VSphereConfig
	cfg.Global.VCenterIP = os.Getenv("VSPHERE_VCENTER")
	cfg.Global.User = os.Getenv("VSPHERE_USER")
	cfg.Global.Password = os.Getenv("VSPHERE_PASSWORD")
	cfg.Global.Datacenter = os.Getenv("VSPHERE_DATACENTER")
	cfg.Global.Datastore = os.Getenv("VSPHERE_DATASTORE")
	cfg.Global.WorkingDir = os.Getenv("VSPHERE_WORKING_DIR")
	cfg.Global.VMName = os.Getenv("VSPHERE_VM_NAME")
	cfg.Global.InsecureFlag = false
	if strings.ToLower(os.Getenv("VSPHERE_INSECURE")) == "true" {
		cfg.Global.InsecureFlag = true
	}
	return &cfg
}

func GetgovmomiClient(cfg *VSphereConfig) (*govmomi.Client, error) {
	if cfg == nil {
		cfg = getVSphereConfig()
	}
	client, err := newClient(context.TODO(), cfg)
	return client, err
}

// Get list of compatible datastores that satisfies the storage policy requirements.
func (vs *VSphere) GetCompatibleDatastores(ctx context.Context, pbmClient *pbm.Client, resourcePool *object.ResourcePool, storagePolicyID string) ([]mo.Datastore, error) {
	datastores, err := vs.getAllAccessibleDatastoresForK8sCluster(ctx, resourcePool)
	if err != nil {
		return nil, err
	}
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
		return nil, err
	}
	compatibleHubs := res.CompatibleDatastores()
	// Return an error if there are no compatible datastores.
	if len(compatibleHubs) < 1 {
		return nil, fmt.Errorf("There are no compatible datastores: %+v that satisfy the storage policy: %+q requirements", datastores, storagePolicyID)
	}
	var compatibleDatastoreRefs []types.ManagedObjectReference
	for _, hub := range compatibleHubs {
		compatibleDatastoreRefs = append(compatibleDatastoreRefs, types.ManagedObjectReference{
			Type:  hub.HubType,
			Value: hub.HubId,
		})
	}
	dsMorefs, err := vs.getDatastoreMorefs(ctx, compatibleDatastoreRefs)
	if err != nil {
		return nil, err
	}
	return dsMorefs, nil
}

// Verify if the user specified datastore is in the list of compatible datastores.
func IsUserSpecifiedDatastoreCompatible(dsRefs []mo.Datastore, dsName string) bool {
	for _, ds := range dsRefs {
		if ds.Info.GetDatastoreInfo().Name == dsName {
			return true
		}
	}
	return false
}

// Get the best fit compatible datastore by free space.
func GetBestFitCompatibleDatastore(dsRefs []mo.Datastore) string {
	var curMax int64
	curMax = -1
	var index int
	for i, ds := range dsRefs {
		dsFreeSpace := ds.Info.GetDatastoreInfo().FreeSpace
		if dsFreeSpace > curMax {
			curMax = dsFreeSpace
			index = i
		}
	}
	return dsRefs[index].Info.GetDatastoreInfo().Name
}

// Get the datastore morefs.
func (vs *VSphere) getDatastoreMorefs(ctx context.Context, dsRefs []types.ManagedObjectReference) ([]mo.Datastore, error) {
	pc := property.DefaultCollector(vs.client.Client)
	var datastoreMorefs []mo.Datastore
	err := pc.Retrieve(ctx, dsRefs, []string{DatastoreInfoProperty}, &datastoreMorefs)
	if err != nil {
		return nil, err
	}
	return datastoreMorefs, nil
}

// Get all datastores accessible inside the current Kubernetes cluster.
func (vs *VSphere) getAllAccessibleDatastoresForK8sCluster(ctx context.Context, resourcePool *object.ResourcePool) ([]types.ManagedObjectReference, error) {
	var resourcePoolMoref mo.ResourcePool
	s := object.NewSearchIndex(vs.client.Client)
	err := s.Properties(ctx, resourcePool.Reference(), []string{ParentProperty}, &resourcePoolMoref)
	if err != nil {
		return nil, err
	}

	// The K8s cluster might be deployed inside a cluster or a host.
	// For a cluster it is ClusterComputeResource object, for others it is a ComputeResource object.
	var datastores []types.ManagedObjectReference
	if resourcePoolMoref.Parent.Type == ClusterComputeResource {
		var cluster mo.ClusterComputeResource
		err = s.Properties(ctx, resourcePoolMoref.Parent.Reference(), []string{DatastoreProperty}, &cluster)
		if err != nil {
			return nil, err
		}
		datastores = cluster.Datastore
	} else {
		var host mo.ComputeResource
		err = s.Properties(ctx, resourcePoolMoref.Parent.Reference(), []string{DatastoreProperty}, &host)
		if err != nil {
			return nil, err
		}
		datastores = host.Datastore
	}
	return datastores, nil
}
