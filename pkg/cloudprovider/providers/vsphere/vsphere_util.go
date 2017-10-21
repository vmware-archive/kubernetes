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
	"errors"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"

	"fmt"

	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib/diskmanagers"
)

const (
	DatastoreProperty     = "datastore"
	DatastoreInfoProperty = "info"
	Folder                = "Folder"
	VirtualMachine        = "VirtualMachine"
)

// GetVSphere reads vSphere configuration from system environment and construct vSphere object
func GetVSphere() (*VSphere, error) {
	cfg := getVSphereConfig()
	vSphereConn := getVSphereConn(cfg)
	client, err := GetgovmomiClient(vSphereConn)
	if err != nil {
		return nil, err
	}
	vSphereConn.GoVmomiClient = client
	vsphereIns := &VSphereInstance{
		conn: vSphereConn,
		cfg: &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			VCenterPort:       cfg.Global.VCenterPort,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
		},
	}
	vsphereInsMap := make(map[string]*VSphereInstance)
	vsphereInsMap[""] = vsphereIns
	// TODO: Initialize nodeManager and set it in VSphere.
	vs := &VSphere{
		vsphereInstanceMap: vsphereInsMap,
		hostName:           "",
		cfg:                cfg,
		nodeManager: &NodeManager{
			vsphereInstanceMap: vsphereInsMap,
		},
	}
	runtime.SetFinalizer(vs, logout)
	return vs, nil
}

func getVSphereConfig() *VSphereConfig {
	var cfg VSphereConfig
	cfg.Global.VCenterIP = os.Getenv("VSPHERE_VCENTER")
	cfg.Global.VCenterPort = os.Getenv("VSPHERE_VCENTER_PORT")
	cfg.Global.User = os.Getenv("VSPHERE_USER")
	cfg.Global.Password = os.Getenv("VSPHERE_PASSWORD")
	cfg.Global.Datacenters = os.Getenv("VSPHERE_DATACENTER")
	cfg.Global.DefaultDatastore = os.Getenv("VSPHERE_DATASTORE")
	cfg.Global.WorkingDir = os.Getenv("VSPHERE_WORKING_DIR")
	cfg.Global.VMName = os.Getenv("VSPHERE_VM_NAME")
	cfg.Global.InsecureFlag = false
	if strings.ToLower(os.Getenv("VSPHERE_INSECURE")) == "true" {
		cfg.Global.InsecureFlag = true
	}
	return &cfg
}

func getVSphereConn(cfg *VSphereConfig) *vclib.VSphereConnection {
	vSphereConn := &vclib.VSphereConnection{
		Username:          cfg.Global.User,
		Password:          cfg.Global.Password,
		Hostname:          cfg.Global.VCenterIP,
		Insecure:          cfg.Global.InsecureFlag,
		RoundTripperCount: cfg.Global.RoundTripperCount,
		Port:              cfg.Global.VCenterPort,
	}
	return vSphereConn
}

// GetgovmomiClient gets the goVMOMI client for the vsphere connection object
func GetgovmomiClient(conn *vclib.VSphereConnection) (*govmomi.Client, error) {
	if conn == nil {
		cfg := getVSphereConfig()
		conn = getVSphereConn(cfg)
	}
	client, err := conn.NewClient(context.TODO())
	return client, err
}

// getvmUUID gets the BIOS UUID via the sys interface.  This UUID is known by vsphere
func getvmUUID() (string, error) {
	id, err := ioutil.ReadFile(UUIDPath)
	if err != nil {
		return "", fmt.Errorf("error retrieving vm uuid: %s", err)
	}
	uuidFromFile := string(id[:])
	//strip leading and trailing white space and new line char
	uuid := strings.TrimSpace(uuidFromFile)
	// check the uuid starts with "VMware-"
	if !strings.HasPrefix(uuid, UUIDPrefix) {
		return "", fmt.Errorf("Failed to match Prefix, UUID read from the file is %v", uuidFromFile)
	}
	// Strip the prefix and while spaces and -
	uuid = strings.Replace(uuid[len(UUIDPrefix):(len(uuid))], " ", "", -1)
	uuid = strings.Replace(uuid, "-", "", -1)
	if len(uuid) != 32 {
		return "", fmt.Errorf("Length check failed, UUID read from the file is %v", uuidFromFile)
	}
	// need to add dashes, e.g. "564d395e-d807-e18a-cb25-b79f65eb2b9f"
	uuid = fmt.Sprintf("%s-%s-%s-%s-%s", uuid[0:8], uuid[8:12], uuid[12:16], uuid[16:20], uuid[20:32])
	return uuid, nil
}

// Get all datastores accessible for the virtual machine object.
func getSharedDatastoresInK8SCluster(ctx context.Context, nodeManager *NodeManager) ([]string, error) {
	vmList := nodeManager.GetNodeVms()
	if vmList == nil || len(vmList) == 0 {
		msg := fmt.Sprintf("Kubernetes node vm list is empty. vmList : %+v", vmList)
		glog.Error(msg)
		return nil, fmt.Errorf(msg)
	}
	index := 0
	var sharedDatastores []string
	for _, vm := range vmList {
		vmName, err := vm.ObjectName(ctx)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(vmName, DummyVMPrefixName) {
			accessibleDatastores, err := vm.GetAllAccessibleDatastores(ctx)
			if err != nil {
				return nil, err
			}
			if index == 0 {
				sharedDatastores = accessibleDatastores
			} else {
				// Note that we are using datastore names for intersection. For multi-vc support, we are assuming that the
				// datastore names are same for the shared storage across datacenters and virtual centers. This is because,
				// the PVs will have the datastore name in the volume path if old version of k8s is used or due to upgrade
				// or due to static provisioning or PVs. Using datastore URLs in PVs, VCP logic, vsphere.config,
				// storage class etc should be thought through more.
				// TODO: THIS IS WRONG! Use datastoreURL for intersection!!!
				// Ideally we should intersect on datastore URLs here.
				sharedDatastores = intersect(sharedDatastores, accessibleDatastores)
				if len(sharedDatastores) == 0 {
					return nil, fmt.Errorf("No shared datastores found in the Kubernetes cluster for vmList: %+v", vmList)
				}
			}
			index++
		}
	}
	return sharedDatastores, nil
}

func intersect(list1 []string, list2 []string) []string {
	glog.V(4).Infof("list1: %+v", list1)
	glog.V(4).Infof("list2: %+v", list2)
	var sharedDs []string
	for _, val1 := range list1 {
		// Check if val1 is found in list2
		for _, val2 := range list2 {
			if val1 == val2 {
				sharedDs = append(sharedDs, val1)
				break
			}
		}
	}
	return sharedDs
}

// Get the datastores accessible for the virtual machine object.
func getAllAccessibleDatastores(ctx context.Context, client *vim25.Client, vmMo mo.VirtualMachine) ([]string, error) {
	host := vmMo.Summary.Runtime.Host
	if host == nil {
		return nil, errors.New("VM doesn't have a HostSystem")
	}
	var hostSystemMo mo.HostSystem
	s := object.NewSearchIndex(client)
	err := s.Properties(ctx, host.Reference(), []string{DatastoreProperty}, &hostSystemMo)
	if err != nil {
		return nil, err
	}
	var dsRefValues []string
	for _, dsRef := range hostSystemMo.Datastore {
		dsRefValues = append(dsRefValues, dsRef.Value)
	}
	return dsRefValues, nil
}

// getMostFreeDatastore gets the best fit compatible datastore by free space.
func getMostFreeDatastoreName(ctx context.Context, client *vim25.Client, dsObjList []*vclib.Datastore) (string, error) {
	dsMoList, err := dsObjList[0].Datacenter.GetDatastoreMoList(ctx, dsObjList, []string{DatastoreInfoProperty})
	if err != nil {
		return "", err
	}
	var curMax int64
	curMax = -1
	var index int
	for i, dsMo := range dsMoList {
		dsFreeSpace := dsMo.Info.GetDatastoreInfo().FreeSpace
		if dsFreeSpace > curMax {
			curMax = dsFreeSpace
			index = i
		}
	}
	return dsMoList[index].Info.GetDatastoreInfo().Name, nil
}

func getDatastoresForEndpointVC(ctx context.Context, dc *vclib.Datacenter, dsNames []string) []*vclib.Datastore {
	var datastores []*vclib.Datastore
	for _, dsName := range dsNames {
		ds, err := dc.GetDatastoreByName(ctx, dsName)
		if err != nil {
			glog.V(4).Infof("Warning: Could not find datastore with name %s in datacenter %s! Ignoring it..", dsName, dc.Name())
			continue
		}
		datastores = append(datastores, ds)
	}
	return datastores
}

func getPbmCompatibleDatastore(ctx context.Context, dc *vclib.Datacenter, storagePolicyName string, nodeManager *NodeManager) (string, error) {
	pbmClient, err := vclib.NewPbmClient(ctx, dc.Client())
	if err != nil {
		return "", err
	}
	storagePolicyID, err := pbmClient.ProfileIDByName(ctx, storagePolicyName)
	if err != nil {
		glog.Errorf("Failed to get Profile ID by name: %s. err: %+v", storagePolicyName, err)
		return "", err
	}
	sharedDs, err := getSharedDatastoresInK8SCluster(ctx, nodeManager)
	if err != nil {
		glog.Errorf("Failed to get shared datastores. err: %+v", err)
		return "", err
	}
	sharedDsList := getDatastoresForEndpointVC(ctx, dc, sharedDs)
	if len(sharedDsList) == 0 {
		msg := "No shared datastores found in the endpoint virtual center"
		glog.Errorf(msg)
		return "", errors.New(msg)
	}
	compatibleDatastores, _, err := pbmClient.GetCompatibleDatastores(ctx, dc, storagePolicyID, sharedDsList)
	if err != nil {
		glog.Errorf("Failed to get compatible datastores from datastores : %+v with storagePolicy: %s. err: %+v", sharedDsList, storagePolicyID, err)
		return "", err
	}
	datastore, err := getMostFreeDatastoreName(ctx, dc.Client(), compatibleDatastores)
	if err != nil {
		glog.Errorf("Failed to get most free datastore from compatible datastores: %+v. err: %+v", compatibleDatastores, err)
		return "", err
	}
	return datastore, err
}

func (vs *VSphere) setVMOptions(ctx context.Context, dc *vclib.Datacenter, computePath string) (*vclib.VMOptions, error) {
	var vmOptions vclib.VMOptions
	resourcePool, err := dc.GetResourcePool(ctx, computePath)
	if err != nil {
		return nil, err
	}
	folder, err := dc.GetFolderByPath(ctx, vs.cfg.Workspace.Folder)
	if err != nil {
		return nil, err
	}
	vmOptions.VMFolder = folder
	vmOptions.VMResourcePool = resourcePool
	return &vmOptions, nil
}

// A background routine which will be responsible for deleting stale dummy VM's.
func (vs *VSphere) cleanUpDummyVMs(dummyVMPrefix string) {
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		time.Sleep(CleanUpDummyVMRoutineInterval * time.Minute)
		vsi, err := vs.getVSphereInstanceForServer(vs.cfg.Workspace.VCenterIP, ctx)
		if err != nil {
			glog.V(4).Infof("Failed to get VSphere instance with err: %+v. Retrying again...", err)
			continue
		}
		dc, err := vclib.GetDatacenter(ctx, vsi.conn, vs.cfg.Workspace.Datacenter)
		if err != nil {
			glog.V(4).Infof("Failed to get the datacenter: %s from VC. err: %+v", vs.cfg.Workspace.Datacenter, err)
			continue
		}
		// Get the folder reference for global working directory where the dummy VM needs to be created.
		vmFolder, err := dc.GetFolderByPath(ctx, vs.cfg.Workspace.Folder)
		if err != nil {
			glog.V(4).Infof("Unable to get the kubernetes folder: %q reference. err: %+v", vs.cfg.Global.WorkingDir, err)
			continue
		}
		// A write lock is acquired to make sure the cleanUp routine doesn't delete any VM's created by ongoing PVC requests.
		defer cleanUpDummyVMLock.Lock()
		err = diskmanagers.CleanUpDummyVMs(ctx, vmFolder, dc)
		if err != nil {
			glog.V(4).Infof("Unable to clean up dummy VM's in the kubernetes cluster: %q. err: %+v", vs.cfg.Global.WorkingDir, err)
		}
	}
}
