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

package photon_pd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/photon"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

const (
	maxRetries         = 10
	checkSleepDuration = time.Second
	diskByIDPath       = "/dev/disk/by-id/"
	diskPhotonPrefix   = "wwn-0x"
)

var ErrProbeVolume = errors.New("Error scanning attached volumes")
var volNameToDeviceName = make(map[string]string)

type PhotonDiskUtil struct{}

func removeFromScsiSubsystem(volName string) {
	// TODO: if using pvscsi controller, this won't be needed
	deviceName := volNameToDeviceName[volName]
	fileName := "/sys/block/" + deviceName + "/device/delete"
	data := []byte("1")
	ioutil.WriteFile(fileName, data, 0666)
}

func scsiHostScan() {
	scsi_path := "/sys/class/scsi_host/"
	if dirs, err := ioutil.ReadDir(scsi_path); err == nil {
		for _, f := range dirs {
			name := scsi_path + f.Name() + "/scan"
			data := []byte("- - -")
			ioutil.WriteFile(name, data, 0666)
			glog.Errorf("scsiHostScan scan for %s", name)
		}
	}
}

func verifyDevicePath(path string) (string, error) {
	if pathExists, err := volumeutil.PathExists(path); err != nil {
		return "", fmt.Errorf("Error checking if path exists: %v", err)
	} else if pathExists {
		return path, nil
	}

	glog.Errorf("Photon Controller verifyDevicePath: not exists")
	return "", nil
}

// CreateVolume creates a PhotonController persistent disk.
func (util *PhotonDiskUtil) CreateVolume(p *photonPersistentDiskProvisioner) (pdID string, capacityGB int, err error) {
	cloud, err := getCloudProvider(p.plugin.host.GetCloudProvider())
	if err != nil {
		return "", 0, err
	}

	capacity := p.options.PVC.Spec.Resources.Requests[api.ResourceName(api.ResourceStorage)]
	volSizeBytes := capacity.Value()
	// PhotonController works with GB, convert to GB with rounding up
	volSizeGB := int(volume.RoundUpSize(volSizeBytes, 1024*1024*1024))
	name := volume.GenerateVolumeName(p.options.ClusterName, p.options.PVName, 255)
	volumeOptions := &photon.VolumeOptions{
		CapacityGB: volSizeGB,
		Tags:       *p.options.CloudTags,
		Name:       name,
	}

	for parameter, value := range p.options.Parameters {
		switch strings.ToLower(parameter) {
		case "flavor":
			volumeOptions.Flavor = value
		default:
			return "", 0, fmt.Errorf("invalid option %q for volume plugin %s", parameter, p.plugin.GetPluginName())
		}
	}

	pdID, err = cloud.CreateDisk(volumeOptions)
	if err != nil {
		glog.V(2).Infof("Error creating PhotonController persistent disk: %v", err)
		return "", 0, err
	}

	glog.V(2).Infof("Successfully created PhotonController persistent disk %s", name)
	return pdID, volSizeGB, nil
}

// DeleteVolume deletes a vSphere volume.
func (util *PhotonDiskUtil) DeleteVolume(pd *photonPersistentDiskDeleter) error {
	cloud, err := getCloudProvider(pd.plugin.host.GetCloudProvider())
	if err != nil {
		return err
	}

	if err = cloud.DeleteDisk(pd.pdID); err != nil {
		glog.V(2).Infof("Error deleting PhotonController persistent disk %s: %v", pd.pdID, err)
		return err
	}
	glog.V(2).Infof("Successfully deleted PhotonController persistent disk %s", pd.pdID)
	return nil
}

func getCloudProvider(cloud cloudprovider.Interface) (*photon.PCCloud, error) {
	if cloud == nil {
		glog.Errorf("Cloud provider not initialized properly")
		return nil, errors.New("Cloud provider not initialized properly")
	}

	pcc := cloud.(*photon.PCCloud)
	if pcc == nil {
		return nil, errors.New("Invalid cloud provider: expected Photon Controller")
	}
	return pcc, nil
}
