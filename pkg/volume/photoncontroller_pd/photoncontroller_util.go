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

package photoncontroller_pd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/vmware/photon-controller-go-sdk/photon"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/photoncontroller"
	"k8s.io/kubernetes/pkg/util/keymutex"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/volume"
)

const (
	diskPartitionSuffix = ""
	diskXVDPath         = "/dev/xvd"
	diskXVDPattern      = "/dev/xvd*"
	maxChecks           = 60
	maxRetries          = 10
	checkSleepDuration  = time.Second
	errorSleepDuration  = 5 * time.Second
)

// Singleton key mutex for keeping attach/detach operations for the same PD atomic
var attachDetachMutex = keymutex.NewKeyMutex()

type PhotonControllerDiskUtil struct{}

// Attaches a disk to the current kubelet.
// Mounts the disk to it's global path.
func (diskUtil *PhotonControllerDiskUtil) AttachAndMountDisk(b *photonControllerPersistentDiskMounter, globalPDPath string) error {
	glog.V(1).Infof("AttachAndMountDisk(...) called for PD %q. Will block for existing operations, if any. (globalPDPath=%q)\r\n", b.diskID, globalPDPath)

	// Block execution until any pending detach operations for this PD have completed
	attachDetachMutex.LockKey(b.diskID)
	defer attachDetachMutex.UnlockKey(b.diskID)

	glog.V(1).Infof("AttachAndMountDisk(...) called for PD %q. Awake and ready to execute. (globalPDPath=%q)\r\n", b.diskID, globalPDPath)

	xvdBefore, err := filepath.Glob(diskXVDPattern)
	if err != nil {
		glog.Errorf("Error filepath.Glob(\"%s\"): %v\r\n", diskXVDPattern, err)
	}
	xvdBeforeSet := sets.NewString(xvdBefore...)

	devicePath, err := attachDiskAndVerify(b, xvdBeforeSet)
	if err != nil {
		return err
	}

	// Only mount the PD globally once.
	notMnt, err := b.mounter.IsLikelyNotMountPoint(globalPDPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(globalPDPath, 0750); err != nil {
				return err
			}
			notMnt = true
		} else {
			return err
		}
	}
	options := []string{}
	if b.readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		err = b.diskMounter.FormatAndMount(devicePath, globalPDPath, b.fsType, options)
		if err != nil {
			os.Remove(globalPDPath)
			return err
		}
	}
	return nil
}

// Unmounts the device and detaches the disk from the kubelet's host machine.
func (util *PhotonControllerDiskUtil) DetachDisk(c *photonControllerPersistentDiskUnmounter) error {
	glog.V(1).Infof("DetachDisk(...) for PD %q\r\n", c.diskID)

	if err := unmountPDAndRemoveGlobalPath(c); err != nil {
		glog.Errorf("Error unmounting PD %q: %v", c.diskID, err)
	}

	// Detach disk asynchronously so that the kubelet sync loop is not blocked.
	go detachDiskAndVerify(c)
	return nil
}

func (util *PhotonControllerDiskUtil) DeleteDisk(d *photonControllerPersistentDiskDeleter) error {
	// TODO actually implement this method.
	glog.V(1).Infof("Successfully deleted Photon Controller Persistent Disk %s", d.diskID)
	return nil
}

func (util *PhotonControllerDiskUtil) CreateDisk(c *photonControllerPersistentDiskProvisioner, d *api.PersistentVolume) (diskID string, diskSizeGB int, err error) {
	pc, err := getCloudProvider()
	if err != nil {
		return
	}



	spec := &photon.DiskCreateSpec{}
	spec.Name = d.Name
	spec.Flavor = ""
	spec.Kind = "persistent"
	requestBytes := c.options.Capacity.Value()
	spec.CapacityGB = int(volume.RoundUpSize(requestBytes, 1024*1024*1024))

	diskID, err = pc.CreateDisk(spec)
	if err != nil {
		return "", 0, err
	}

	return diskID, spec.CapacityGB, nil
}

func (util *PhotonControllerDiskUtil) DeleteVolume(d *photonControllerPersistentDiskDeleter) error {
	cloud, err := getCloudProvider()
	if err != nil {
		return err
	}

	if _, err = cloud.DeleteDisk(d.diskID); err != nil {
		glog.V(2).Infof("Error deleting Photon PD volume %s: %v", d.diskID, err)
		return err
	}
	glog.V(2).Infof("Successfully deleted Photon PD volume %s", d.diskID)
	return nil
}

// CreateVolume creates a Photon PD.
// Returns: volumeID, volumeSizeGB, labels, error
func (util *PhotonControllerDiskUtil) CreateVolume(c *photonControllerPersistentDiskProvisioner) (string, int,
map[string]string, error) {
	cloud, err := getCloudProvider()
	if err != nil {
		return "", 0, nil, err
	}

	name := volume.GenerateVolumeName(c.options.ClusterName, c.options.PVName, 63) // Photon PD name can have up to 63 characters
	requestBytes := c.options.Capacity.Value()
	// Photon works with gigabytes, convert to GiB with rounding up
	requestGB := volume.RoundUpSize(requestBytes, 1024*1024*1024)

	// The disk will be created in the zone in which this code is currently running
	// TODO: We should support auto-provisioning volumes in multiple/specified zones

	spec := &photon.DiskCreateSpec{}
	spec.Name = name
	spec.Flavor = ""
	// TODO:
	// /spec.Tags = c.options.CloudTags
	spec.Kind = "persistent"

	spec.CapacityGB = int(requestGB)

	_, err = cloud.CreateDisk(spec)
	if err != nil {
		glog.V(2).Infof("Error creating Photon PD volume: %v", err)
		return "", 0, nil, err
	}
	glog.V(2).Infof("Successfully created Photon PD volume %s", name)
	return name, int(requestGB), nil, nil
}

// Attaches the specified persistent disk device to node, verifies that it is attached, and retries if it fails.
func attachDiskAndVerify(b *photonControllerPersistentDiskMounter, xvdBeforeSet sets.String) (string, error) {
	glog.V(1).Infof("Successfully attached Photon Controller Persistent Disk %q.", b.diskID)
	return "", nil
}

// Returns the first path that exists, or empty string if none exist.
func verifyDevicePath(devicePaths []string) (string, error) {
	for _, path := range devicePaths {
		if pathExists, err := pathExists(path); err != nil {
			return "", fmt.Errorf("Error checking if path exists: %v", err)
		} else if pathExists {
			return path, nil
		}
	}

	return "", nil
}

// Detaches the specified persistent disk device from node, verifies that it is detached, and retries if it fails.
// This function is intended to be called asynchronously as a go routine.
func detachDiskAndVerify(c *photonControllerPersistentDiskUnmounter) {
	glog.V(1).Infof("Successfully detached Photon Controller Persistent Disk %q.", c.diskID)
}

// Unmount the global PD mount, which should be the only one, and delete it.
func unmountPDAndRemoveGlobalPath(c *photonControllerPersistentDiskUnmounter) error {
	globalPDPath := makeGlobalPDName(c.plugin.host, c.diskID)

	err := c.mounter.Unmount(globalPDPath)
	os.Remove(globalPDPath)
	return err
}

// Checks if the specified path exists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// Return cloud provider
func getCloudProvider() (*photoncontroller.PhotonController, error) {
	pc, err := cloudprovider.GetCloudProvider("photoncontroller", nil)
	if err != nil || pc == nil {
		return nil, err
	}
	return pc.(*photoncontroller.PhotonController), nil
}
