/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package photoncontroller

import (
	"io"
	"fmt"
	"errors"

	"github.com/golang/glog"
	"github.com/vmware/photon-controller-go-sdk/photon"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/api"

	"gopkg.in/gcfg.v1"
)

const ProviderName = "photoncontroller"

// PhotonController is an implementation of cloud provider Interface for Photon Controller.
type PhotonController struct {
    client *photon.Client
	vmID                   string
    projectID              string
}

type Config struct {
	Global struct {
		ApiEndpoint string `gcfg:"api-endpoint"`
		ProjectID   string `gcfg:"project-id"`
	}
}

func init() {
	glog.V(1).Info("init called of PhotonController")
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return newPhotonController(cfg)
	})
}

func readConfig(config io.Reader) (Config, error) {
	glog.V(1).Info("readConfig called of PhotonController")
	if config == nil {
		// TODO Only rely on the cfg file
        //	err := fmt.Errorf("no Photon Controller cloud provider config file given")
        // return Config{}, err
		cfg := Config{}
		cfg.Global.ApiEndpoint = "http://10.146.56.2:9000"
		cfg.Global.ProjectID = "83480dab-4983-4bc5-9dbd-87afd61e7b39"
		return cfg, nil
 	}

	var cfg Config
	err := gcfg.ReadInto(&cfg, config)
	return cfg, err
}

func newPhotonController(cfg Config) (*PhotonController, error) {
	client := photon.NewClient(cfg.Global.ApiEndpoint, nil, nil)
	vmID, err := readVmID()
	if err != nil {
		return nil, err
	}
	pd := PhotonController{
		client:        client,
		projectID:     cfg.Global.ProjectID,
		vmID:          vmID,
	}
	return &pd, nil
}

func readVmID() (string, error) {
	glog.V(1).Info("readVmID called of PhotonController")
	return "vmID", nil
}

// ProviderName returns the cloud provider ID.
func (pd *PhotonController) ProviderName() string {
	glog.V(1).Info("ProviderName called of PhotonController")
	return ProviderName
}

func (pd *PhotonController) Clusters() (cloudprovider.Clusters, bool) {
	glog.V(1).Info("Clusters called of PhotonController")
	return nil, false
}

// ScrubDNS filters DNS settings for pods.
func (pd *PhotonController) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	glog.V(1).Info("ScrubDNS called of PhotonController")
	return nameservers, searches
}

func (pd *PhotonController) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	glog.V(1).Info("LoadBalancer called of PhotonController")
	return nil, false
}

func (pd *PhotonController) Zones() (cloudprovider.Zones, bool) {
	glog.V(1).Info("Zones called of PhotonController")
	return nil, false
}

func (pd *PhotonController) Instances() (cloudprovider.Instances, bool) {
	glog.V(1).Info("Instances called of PhotonController")
	return pd, true
}

func (pd *PhotonController) GetZone() (cloudprovider.Zone, error) {
	glog.V(1).Info("GetZone called of PhotonController")
	var zone cloudprovider.Zone = cloudprovider.Zone{}
	return zone, nil
}

func (pd *PhotonController) Routes() (cloudprovider.Routes, bool) {
	glog.V(1).Info("Routes called of PhotonController")
	return nil, false
}

func (pd *PhotonController) NodeAddresses(name string) ([]api.NodeAddress, error) {
	glog.V(1).Info("NodeAddresses called of PhotonController")
	return make([]api.NodeAddress, 0), nil
}

func (pd *PhotonController) ExternalID(name string) (string, error) {
	glog.V(1).Info("ExternalID called of PhotonController")
	return name, nil
}

func (pd *PhotonController) InstanceID(name string) (string, error) {
	glog.V(1).Info("InstanceID called of PhotonController")
	return name, nil
}

func (pd *PhotonController) InstanceType(name string) (string, error) {
	glog.V(1).Info("InstanceType called of PhotonController")
	return name, nil
}

func (pd *PhotonController) List(filter string) ([]string, error) {
	glog.V(1).Info("List called of PhotonController")
	return make([]string, 0), nil
}

func (pd *PhotonController) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	glog.V(1).Info("AddSSHKeyToAllInstances called of PhotonController")
	return nil
}

func (pd *PhotonController) CurrentNodeName(hostname string) (string, error) {
	glog.V(1).Info("CurrentNodeName called of PhotonController")
	return hostname, nil
}

func (pd *PhotonController) CreateDisk(spec *photon.DiskCreateSpec) (string, error) {
	glog.V(1).Info("Creating disk in Photon Controller")
	if spec == nil {
		return "", errors.New("Passed diskCreateSpec is nil")
	}

	glog.V(1).Infof(
		"disk name: %s, capacity: %s, flavor: %s, kind: %s.",
		spec.Name,
		spec.CapacityGB,
		spec.Flavor,
		spec.Kind)


	task, err := pd.client.Projects.CreateDisk(pd.projectID, spec)
	if err != nil {
		return "", fmt.Errorf("error creating Disk in Photon Controller: %v", err)
	}

	task, err = pd.client.Tasks.Wait(task.ID)
	if err != nil {
		return "", fmt.Errorf("Create Disk task failed: %v", err)
	}

	return task.Entity.ID, nil
}

func (pd *PhotonController) DeleteDisk(diskId string) (string, error) {
	glog.V(1).Infof("Deleting disk from Photon Controller: %s", diskId)
	task, err := pd.client.Disks.Delete(diskId)
	if err != nil {
		return "", fmt.Errorf("error deleting Disk in Photon Controller: %v", err)
	}

	task, err = pd.client.Tasks.Wait(task.ID)
	if err != nil {
		return "", fmt.Errorf("Delete Disk task failed: %v", err)
	}

	return task.Entity.ID, nil
}
