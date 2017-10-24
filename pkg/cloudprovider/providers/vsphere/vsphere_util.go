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
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
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

func getAccessibleDatastores(ctx context.Context, nodeVmDetail *NodeDetails, nodeManager *NodeManager) ([]*vclib.DatastoreInfo, error) {
	accessibleDatastores, err := nodeVmDetail.vm.GetAllAccessibleDatastores(ctx)
	if err != nil {
		if IsManagedObjectNotFoundError(err) {
			glog.V(4).Infof("error %q ManagedObjectNotFound for node %q. Rediscovering...", err, nodeVmDetail.NodeName)
			err = nodeManager.RediscoverNode(convertToK8sType(nodeVmDetail.NodeName))
			if err == nil {
				glog.V(4).Infof("Discovered node %s successfully", nodeVmDetail.NodeName)
				nodeInfo, err := nodeManager.GetNodeInfo(convertToK8sType(nodeVmDetail.NodeName))
				if err != nil {
					glog.V(4).Infof("error %q getting node info for node %+v", err, nodeVmDetail)
					return nil, err
				}

				accessibleDatastores, err = nodeInfo.vm.GetAllAccessibleDatastores(ctx)
				if err != nil {
					glog.V(4).Infof("error %q getting accessible datastores for node %+v", err, nodeVmDetail)
					return nil, err
				}
			} else {
				glog.V(4).Infof("error %q rediscovering node %+v", err, nodeVmDetail)
				return nil, err
			}
		} else {
			glog.V(4).Infof("error %q getting accessible datastores for node %+v", err, nodeVmDetail)
			return nil, err
		}
	}
	return accessibleDatastores, nil
}

// Get all datastores accessible for the virtual machine object.
func getSharedDatastoresInK8SCluster(ctx context.Context, dc *vclib.Datacenter, nodeManager *NodeManager) ([]*vclib.DatastoreInfo, error) {
	nodeVmDetails := nodeManager.GetNodeDetails()
	if nodeVmDetails == nil || len(nodeVmDetails) == 0 {
		msg := fmt.Sprintf("Kubernetes node nodeVmDetail details is empty. nodeVmDetails : %+v", nodeVmDetails)
		glog.Error(msg)
		return nil, fmt.Errorf(msg)
	}
	index := 0
	var sharedDatastores []*vclib.DatastoreInfo
	for _, nodeVmDetail := range nodeVmDetails {
		glog.V(4).Infof("Getting accessible datastores for node %s", nodeVmDetail.NodeName)
		accessibleDatastores, err := getAccessibleDatastores(ctx, &nodeVmDetail, nodeManager)
		if err != nil {
			return nil, err
		}
		if index == 0 {
			sharedDatastores = accessibleDatastores
		} else {
			sharedDatastores = intersect(sharedDatastores, accessibleDatastores)
			if len(sharedDatastores) == 0 {
				return nil, fmt.Errorf("No shared datastores found in the Kubernetes cluster for nodeVmDetails: %+v", nodeVmDetails)
			}
		}
		index++
	}
	glog.V(4).Infof("sharedDatastores : %+v", sharedDatastores)
	sharedDatastores, err := getDatastoresForEndpointVC(ctx, dc, sharedDatastores)
	if err != nil {
		glog.Errorf("Failed to get shared datastores from endpoint VC. err: %+v", err)
		return nil, err
	}
	glog.V(4).Infof("sharedDatastores at endpoint VC: %+v", sharedDatastores)
	return sharedDatastores, nil
}

func intersect(list1 []*vclib.DatastoreInfo, list2 []*vclib.DatastoreInfo) []*vclib.DatastoreInfo {
	// TODO: Remove these logs
	glog.V(4).Infof("list1: %+v", list1)
	glog.V(4).Infof("list2: %+v", list2)
	var sharedDs []*vclib.DatastoreInfo
	for _, val1 := range list1 {
		// Check if val1 is found in list2
		for _, val2 := range list2 {
			// Intersect is performed based on the datastoreUrl as this uniquely identifies the datastore.
			if val1.Info.Url == val2.Info.Url {
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
func getMostFreeDatastoreName(ctx context.Context, client *vim25.Client, dsInfoList []*vclib.DatastoreInfo) (string, error) {
	var curMax int64
	curMax = -1
	var index int
	for i, dsInfo := range dsInfoList {
		dsFreeSpace := dsInfo.Info.GetDatastoreInfo().FreeSpace
		if dsFreeSpace > curMax {
			curMax = dsFreeSpace
			index = i
		}
	}
	return dsInfoList[index].Info.GetDatastoreInfo().Name, nil
}

func getDatastoresForEndpointVC(ctx context.Context, dc *vclib.Datacenter, sharedDsInfos []*vclib.DatastoreInfo) ([]*vclib.DatastoreInfo, error) {
	var datastores []*vclib.DatastoreInfo
	allDsInfoMap, err := dc.GetAllDatastores(ctx)
	if err != nil {
		return nil, err
	}
	for _, sharedDsInfo := range sharedDsInfos {
		dsInfo, ok := allDsInfoMap[sharedDsInfo.Info.Url]
		if ok {
			datastores = append(datastores, dsInfo)
		} else {
			glog.V(4).Infof("Warning: Shared datastore with URL %s does not exist in endpoint VC", sharedDsInfo.Info.Url)
		}
	}
	glog.V(4).Infof("Datastore from endpoint VC: %+v", datastores)
	return datastores, nil
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
	sharedDs, err := getSharedDatastoresInK8SCluster(ctx, dc, nodeManager)
	if err != nil {
		glog.Errorf("Failed to get shared datastores. err: %+v", err)
		return "", err
	}
	if len(sharedDs) == 0 {
		msg := "No shared datastores found in the endpoint virtual center"
		glog.Errorf(msg)
		return "", errors.New(msg)
	}
	compatibleDatastores, _, err := pbmClient.GetCompatibleDatastores(ctx, dc, storagePolicyID, sharedDs)
	if err != nil {
		glog.Errorf("Failed to get compatible datastores from datastores : %+v with storagePolicy: %s. err: %+v",
			sharedDs, storagePolicyID, err)
		return "", err
	}
	// TODO: remove this log
	glog.V(4).Infof("compatibleDatastores : %+v", compatibleDatastores)
	datastore, err := getMostFreeDatastoreName(ctx, dc.Client(), compatibleDatastores)
	if err != nil {
		glog.Errorf("Failed to get most free datastore from compatible datastores: %+v. err: %+v", compatibleDatastores, err)
		return "", err
	}
	// TODO: remove this log
	glog.V(4).Infof("Most free datastore : %+s", datastore)
	return datastore, err
}

func (vs *VSphere) setVMOptions(ctx context.Context, dc *vclib.Datacenter, computePath string) (*vclib.VMOptions, error) {
	var vmOptions vclib.VMOptions
	resourcePool, err := dc.GetResourcePool(ctx, computePath)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("Compute path %s, resourcePool %+v", computePath, resourcePool)
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

func IsManagedObjectNotFoundError(err error) bool {
	isManagedObjectNotFoundError := false
	if soap.IsSoapFault(err) {
		_, isManagedObjectNotFoundError = soap.ToSoapFault(err).VimFault().(types.ManagedObjectNotFound)
	}
	return isManagedObjectNotFoundError
}
