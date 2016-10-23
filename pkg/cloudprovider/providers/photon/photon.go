/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package photon

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/vmware/photon-controller-go-sdk/photon"
	"gopkg.in/gcfg.v1"
	"io"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/cloudprovider"
	k8stypes "k8s.io/kubernetes/pkg/types"
	"log"
	"os/exec"
	"strings"
)

const (
	ProviderName = "photon"
)

// Global variable pointing to photon client, can be assigned to mock client in tests
var Esxclient *photon.Client
var logger *log.Logger = nil
var overrideIP bool = false

// Photon is an implementation of cloud provider Interface for Photon Controller.
type PCCloud struct {
	cfg *PCConfig
	// InstanceID of the server where this PCCloud object is instantiated.
	localInstanceID string
	// local $HOSTNAME
	localHostname string
	// hostname from K8S, could be overridden
	localK8sHostname string
	// Photon Project ID
	projID string
	//	instance      *Instances
	cloudprovider.Zone
}

type PCConfig struct {
	Global struct {
		CloudTarget       string `gcfg:"target"`
		IgnoreCertificate bool   `gcfg:"ignoreCertificate"`
		Tenant            string `gcfg:"tenant"`
		Project           string `gcfg:"project"`
		OverrideIP        bool   `gcfg:"overrideIP"`
	}
}

// Disks is interface for manipulation with PhotonController Persistent Disks.
type Disks interface {
	// AttachDisk attaches given disk to given node. Current node
	// is used when nodeName is empty string.
	AttachDisk(pdID string, nodeName k8stypes.NodeName) error

	// DetachDisk detaches given disk to given node. Current node
	// is used when nodeName is empty string.
	DetachDisk(pdID string, nodeName k8stypes.NodeName) error

	// DiskIsAttached checks if a disk is attached to the given node.
	DiskIsAttached(pdID string, nodeName k8stypes.NodeName) (bool, error)

	// CreateDisk creates a new PD with given properties.
	CreateDisk(volumeOptions *VolumeOptions) (pdID string, err error)

	// DeleteDisk deletes PD.
	DeleteDisk(pdID string) error
}

// VolumeOptions specifies capacity, tags, name and flavorID for a volume.
type VolumeOptions struct {
	CapacityGB int
	Tags       map[string]string
	Name       string
	Flavor     string
}

func readConfig(config io.Reader) (PCConfig, error) {
	if config == nil {
		err := fmt.Errorf("no Photon Cloud Provider config file given")
		return PCConfig{}, err
	}

	var cfg PCConfig
	err := gcfg.ReadInto(&cfg, config)
	return cfg, err
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return newPCCloud(cfg)
	})
}

func getVMIDbyNodename(project string, nodeName string) (string, error) {
	vmList, err := Esxclient.Projects.GetVMs(project, nil)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to GetVMs from project %s with nodeName %s", project, nodeName)
		return "", err
	}

	for _, vm := range vmList.Items {
		if vm.Name == nodeName {
			// TODO: check vm state
			return vm.ID, nil
		}
	}

	return "", fmt.Errorf("Photon Cloud Provider: no matching VM is found with name %s", nodeName)
}

func getVMIDbyIP(project string, IPAddress string) (string, error) {
	vmList, err := Esxclient.Projects.GetVMs(project, nil)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to GetVMs from project %s, IP %s", project, IPAddress)
		return "", err
	}

	for _, vm := range vmList.Items {
		task, err := Esxclient.VMs.GetNetworks(vm.ID)
		if err != nil {
			glog.V(2).Info("Photon Cloud Provider: GetNetworks failed for vm.ID %s")
		} else {
			task, err = Esxclient.Tasks.Wait(task.ID)
			if err != nil {
				glog.V(2).Info("Photon Cloud Provider: , Wait task for GetNetworks failed for vm.ID %s")
			} else {
				networkConnections := task.ResourceProperties.(map[string]interface{})
				networks := networkConnections["networkConnections"].([]interface{})
				for _, nt := range networks {
					network := nt.(map[string]interface{})
					if val, ok := network["ipAddress"]; ok && val != nil {
						ipAddr := val.(string)
						if ipAddr == IPAddress {
							return vm.ID, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("Photon Cloud Provider: no matching VM is found with IP %s", IPAddress)
}

func getProjIDbyName(tenantName, projName string) (string, error) {
	tenants, err := Esxclient.Tenants.GetAll()
	if err != nil {
		return "", err
	}

	for _, tenant := range tenants.Items {
		if tenant.Name == tenantName {
			projects, err := Esxclient.Tenants.GetProjects(tenant.ID, nil)
			if err != nil {
				return "", err
			}

			for _, project := range projects.Items {
				if project.Name == projName {
					return project.ID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Photon Cloud Provider: no matching tenant/project name is found with %s/%s", tenantName, projName)
}

func newPCCloud(cfg PCConfig) (*PCCloud, error) {
	if len(cfg.Global.CloudTarget) == 0 {
		return nil, errors.New("Invalid Photon Controller endpoint")
	}

	//TODO: add handling of certification enabled situation
	options := &photon.ClientOptions{
		IgnoreCertificate: cfg.Global.IgnoreCertificate,
	}

	Esxclient = photon.NewClient(cfg.Global.CloudTarget, options, logger)
	status, err := Esxclient.Status.Get()
	if err != nil {
		return nil, errors.New("Photon Controller new client creation failed")
	}
	glog.V(2).Info("Photon Cloud Provider: Status of the new photon controller client: %v", status)

	// Get Photon Controller project ID for future use
	projID, err := getProjIDbyName(cfg.Global.Tenant, cfg.Global.Project)
	if err != nil {
		return nil, err
	}

	// Get local hostname for localInstanceID
	cmd := exec.Command("bash", "-c", `echo $HOSTNAME`)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("Photon Cloud Provider: unable to retrieve hostname for Instance ID")
	}
	hostname := strings.TrimRight(out.String(), "\n")
	vmID, err := getVMIDbyNodename(projID, hostname)
	if err != nil {
		return nil, err
	}

	pc := PCCloud{
		cfg:              &cfg,
		localInstanceID:  vmID,
		localHostname:    hostname,
		localK8sHostname: "",
		projID:           projID,
	}

	overrideIP = cfg.Global.OverrideIP

	return &pc, nil
}

// Instances returns an implementation of Instances for Photon Controller.
func (pc *PCCloud) Instances() (cloudprovider.Instances, bool) {
	return pc, true
}

// List is an implementation of Instances.List.
func (pc *PCCloud) List(filter string) ([]k8stypes.NodeName, error) {
	return nil, nil
}

// NodeAddresses is an implementation of Instances.NodeAddresses.
func (pc *PCCloud) NodeAddresses(nodeName k8stypes.NodeName) ([]api.NodeAddress, error) {
	addrs := []api.NodeAddress{}
	return addrs, nil
}

func (pc *PCCloud) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("unimplemented")
}

func (pc *PCCloud) CurrentNodeName(hostname string) (k8stypes.NodeName, error) {
	pc.localK8sHostname = hostname
	return k8stypes.NodeName(hostname), nil
}

// ExternalID returns the cloud provider ID of the specified instance (deprecated).
func (pc *PCCloud) ExternalID(nodeName k8stypes.NodeName) (string, error) {
	return "ExtID", nil
}

func getInstanceID(name string, projID string) (string, error) {
	var vmID string
	var err error

	if overrideIP == true {
		vmID, err = getVMIDbyIP(projID, name)
	} else {
		vmID, err = getVMIDbyNodename(projID, name)
	}
	if err != nil {
		return "", fmt.Errorf("PhotonController Cloud Provider: failed to getInstanceID %s", name)
	}
	return vmID, nil
}

// InstanceID returns the cloud provider ID of the specified instance.
func (pc *PCCloud) InstanceID(nodeName k8stypes.NodeName) (string, error) {
	name := string(nodeName)
	if name == pc.localK8sHostname {
		return pc.localInstanceID, nil
	} else {
		ID, err := getInstanceID(name, pc.projID)
		return ID, err
	}
}

func (pc *PCCloud) InstanceType(nodeName k8stypes.NodeName) (string, error) {
	return "", nil
}

func (pc *PCCloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, true
}

// ProviderName returns the cloud provider ID.
func (pc *PCCloud) ProviderName() string {
	return ProviderName
}

// LoadBalancer returns an implementation of LoadBalancer for Photon Controller.
func (pc *PCCloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Zones returns an implementation of Zones for Photon Controller.
func (pc *PCCloud) Zones() (cloudprovider.Zones, bool) {
	glog.V(4).Info("Claiming to support Zones")
	return pc, true
}

func (pc *PCCloud) GetZone() (cloudprovider.Zone, error) {
	return pc.Zone, nil
}

// Routes returns a false since the interface is not supported for photon controller.
func (pc *PCCloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ScrubDNS filters DNS settings for pods.
func (pc *PCCloud) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nameservers, searches
}

// Attaches given virtual disk volume to the compute running kubelet.
func (pc *PCCloud) AttachDisk(pdID string, nodeName k8stypes.NodeName) error {
	name := string(nodeName)
	operation := &photon.VmDiskOperation{
		DiskID: pdID,
	}

	var vmID string
	var err error
	if name == pc.localK8sHostname {
		vmID = pc.localInstanceID
	} else {
		vmID, err = getInstanceID(name, pc.projID)
		if err != nil {
			return err
		}
	}

	task, err := Esxclient.VMs.AttachDisk(vmID, operation)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to attach disk")
		return err
	}

	_, err = Esxclient.Tasks.Wait(task.ID)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to wait for task to attach disk")
		return err
	}

	return nil
}

// Detaches given virtual disk volume from the compute running kubelet.
func (pc *PCCloud) DetachDisk(pdID string, nodeName k8stypes.NodeName) error {
	name := string(nodeName)
	operation := &photon.VmDiskOperation{
		DiskID: pdID,
	}

	var vmID string
	var err error
	if name == pc.localK8sHostname {
		// local ID
		vmID = pc.localInstanceID
	} else {
		vmID, err = getInstanceID(name, pc.projID)
		if err != nil {
			return err
		}
	}

	task, err := Esxclient.VMs.DetachDisk(vmID, operation)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to detach disk")
		return err
	}

	_, err = Esxclient.Tasks.Wait(task.ID)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to wait for task to detach disk")
		return err
	}

	return nil
}

// DiskIsAttached returns if disk is attached to the VM using controllers supported by the plugin.
func (pc *PCCloud) DiskIsAttached(pdID string, nodeName k8stypes.NodeName) (bool, error) {
	name := string(nodeName)
	disk, err := Esxclient.Disks.Get(pdID)
	if err != nil {
		return false, err
	}

	var vmID string
	if name == pc.localK8sHostname {
		// local ID
		vmID = pc.localInstanceID
	} else {
		vmID, err = getInstanceID(name, pc.projID)
		if err != nil {
			return false, err
		}
	}

	for _, vm := range disk.VMs {
		if strings.Compare(vm, vmID) == 0 {
			return true, nil
		}
	}

	return false, nil
}

// Create a volume of given size (in GB).
func (pc *PCCloud) CreateDisk(volumeOptions *VolumeOptions) (pdID string, err error) {
	diskSpec := photon.DiskCreateSpec{}
	diskSpec.Name = volumeOptions.Name
	diskSpec.Flavor = volumeOptions.Flavor
	diskSpec.CapacityGB = volumeOptions.CapacityGB
	diskSpec.Kind = "persistent-disk"

	task, err := Esxclient.Projects.CreateDisk(pc.projID, &diskSpec)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to create disk")
		return "", err
	}

	waitTask, err := Esxclient.Tasks.Wait(task.ID)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to wait for task to create disk")
		return "", err
	}

	return waitTask.Entity.ID, nil
}

// Deletes a volume given volume name.
func (pc *PCCloud) DeleteDisk(pdID string) error {
	task, err := Esxclient.Disks.Delete(pdID)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to delete disk")
		return err
	}

	_, err = Esxclient.Tasks.Wait(task.ID)
	if err != nil {
		glog.V(2).Info("Photon Cloud Provider: Failed to wait for task to delete disk")
		return err
	}

	return nil
}
