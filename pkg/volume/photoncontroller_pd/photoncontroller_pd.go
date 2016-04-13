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
	"path"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume"
	"strconv"
	"k8s.io/kubernetes/pkg/util/exec"
)

// This is the primary entrypoint for volume plugins.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&photonControllerPersistentDiskPlugin{nil}}
}

type photonControllerPersistentDiskPlugin struct {
	host volume.VolumeHost
}

var _ volume.VolumePlugin = &photonControllerPersistentDiskPlugin{}
var _ volume.PersistentVolumePlugin = &photonControllerPersistentDiskPlugin{}
var _ volume.DeletableVolumePlugin = &photonControllerPersistentDiskPlugin{}
var _ volume.ProvisionableVolumePlugin = &photonControllerPersistentDiskPlugin{}

const (
	photonControllerPersistentDiskPluginName = "kubernetes.io/photoncontroller-pd"
)

func (plugin *photonControllerPersistentDiskPlugin) Init(host volume.VolumeHost) error {
	plugin.host = host
	return nil
}

func (plugin *photonControllerPersistentDiskPlugin) Name() string {
	return photonControllerPersistentDiskPluginName
}

func (plugin *photonControllerPersistentDiskPlugin) CanSupport(spec *volume.Spec) bool {
	return (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.PhotonControllerDisk != nil) ||
		(spec.Volume != nil && spec.Volume.PhotonControllerDisk != nil)
}

func (plugin *photonControllerPersistentDiskPlugin) GetAccessModes() []api.PersistentVolumeAccessMode {
	return []api.PersistentVolumeAccessMode{
		api.ReadWriteOnce,
	}
}

func (plugin *photonControllerPersistentDiskPlugin) NewDeleter(spec *volume.Spec) (volume.Deleter, error) {
	return plugin.newDeleterInternal(spec, &PhotonControllerDiskUtil{})
}

func (plugin *photonControllerPersistentDiskPlugin) newDeleterInternal(spec *volume.Spec, manager photonControllerManager) (volume.Deleter, error) {
	if spec.PersistentVolume != nil && spec.PersistentVolume.Spec.PhotonControllerDisk== nil {
		return nil, fmt.Errorf("spec.PersistentVolumeSource.PhotonControllerDisk is nil")
	}
	return &photonControllerPersistentDiskDeleter{
		photonPersistentDisk: &photonPersistentDisk{
			volName:  spec.Name(),
			diskID:   spec.PersistentVolume.Spec.PhotonControllerDisk.DiskID,
			manager:  manager,
			plugin:   plugin,
		}}, nil
}

func (plugin *photonControllerPersistentDiskPlugin) NewProvisioner(options volume.VolumeOptions) (volume.Provisioner, error) {
	if len(options.AccessModes) == 0 {
		options.AccessModes = plugin.GetAccessModes()
	}
	return plugin.newProvisionerInternal(options, &PhotonControllerDiskUtil{})
}

func (plugin *photonControllerPersistentDiskPlugin) newProvisionerInternal(options volume.VolumeOptions, manager photonControllerManager) (volume.Provisioner, error) {
	return &photonControllerPersistentDiskProvisioner{
		photonPersistentDisk: &photonPersistentDisk{
			manager: manager,
			plugin:  plugin,
		},
		options: options}, nil
}

func (plugin *photonControllerPersistentDiskPlugin) NewMounter(spec *volume.Spec, pod *api.Pod, _ volume.VolumeOptions) (volume.Mounter, error) {
	// Inject real implementations here, test through the internal function.
	return plugin.newMounterInternal(spec, pod.UID, &PhotonControllerDiskUtil{}, plugin.host.GetMounter())
}

func (plugin *photonControllerPersistentDiskPlugin) newMounterInternal(spec *volume.Spec, podUID types.UID, manager photonControllerManager, mounter mount.Interface) (volume.Mounter, error) {
	// TODO:
	// PhotonControllerPDs used directly in a pod have a ReadOnly flag set by the pod author.
	// PhotonControllerPDs used as a PersistentVolume gets the ReadOnly flag indirectly through the persistent-claim volume used to mount the PV
	var readOnly bool

	var photon *api.PhotonControllerPersistentDiskSource
	if spec.Volume != nil && spec.Volume.PhotonControllerDisk != nil {
		photon = spec.Volume.PhotonControllerDisk
		readOnly = photon.ReadOnly
	} else {
		photon = spec.PersistentVolume.Spec.PhotonControllerDisk
		readOnly = spec.ReadOnly
	}

	diskID := photon.DiskID
	fsType := photon.FSType
	partition := ""
	if photon.Partition != 0 {
		partition = strconv.FormatInt(int64(photon.Partition), 10)
	}

	return &photonControllerPersistentDiskMounter{
		photonPersistentDisk: &photonPersistentDisk{
			podUID:    podUID,
			volName:   spec.Name(),
			diskID:    diskID,
			partition: partition,
			mounter:   mounter,
			manager:   manager,
			plugin:    plugin,
		},
		fsType:      fsType,
		readOnly:    readOnly,
		diskMounter: &mount.SafeFormatAndMount{Interface: mounter, Runner: exec.New()}}, nil
}

func (plugin *photonControllerPersistentDiskPlugin) NewUnmounter(volName string, podUID types.UID) (volume.Unmounter, error) {
	// Inject real implementations here, test through the internal function.
	return plugin.newUnmounterInternal(volName, podUID, &PhotonControllerDiskUtil{}, plugin.host.GetMounter())
}

func (plugin *photonControllerPersistentDiskPlugin) newUnmounterInternal(volName string, podUID types.UID, manager photonControllerManager, mounter mount.Interface) (volume.Unmounter, error) {
	return &photonControllerPersistentDiskUnmounter{&photonPersistentDisk{
		podUID:  podUID,
		volName: volName,
		manager: manager,
		mounter: mounter,
		plugin:  plugin,
	}}, nil
}


// Abstract interface to PD operations.
type photonControllerManager interface {
	// Attaches the disk to the kubelet's host machine.
	AttachAndMountDisk(b *photonControllerPersistentDiskMounter, globalPDPath string) error
	// Detaches the disk from the kubelet's host machine.
	DetachDisk(c *photonControllerPersistentDiskUnmounter) error
	// Creates a disk
	CreateDisk(provisioner *photonControllerPersistentDiskProvisioner, disk *api.PersistentVolume) (diskID string, diskSizeGB int, err error)
	// Deletes a disk
	DeleteDisk(deleter *photonControllerPersistentDiskDeleter) error
}

// photonPersistentDisk are disk resources provided by Photon-Controller
// that are attached to the kubelet's host machine and exposed to the pod.
type photonPersistentDisk struct {
	volName string
	podUID  types.UID
	// Unique id of the PD, used to find the disk resource in the provider.
	diskID string
	// Specifies the partition to mount
	partition string
	// Utility interface that provides API calls to the provider to attach/detach disks.
	manager photonControllerManager
	// Mounter interface that provides system calls to mount the global path to the pod local path.
	mounter mount.Interface
	plugin  *photonControllerPersistentDiskPlugin
	volume.MetricsNil
}

func detachDiskLogError(disk *photonPersistentDisk) {
	err := disk.manager.DetachDisk(&photonControllerPersistentDiskUnmounter{disk})
	if err != nil {
		glog.Warningf("Failed to detach disk: %v (%v)", disk, err)
	}
}

type photonControllerPersistentDiskBuilder struct {
	*photonPersistentDisk
	// Filesystem type, optional.
	fsType string
	// Specifies whether the disk will be attached as read-only.
	readOnly bool
	// diskMounter provides the interface that is used to mount the actual block device.
	diskMounter *mount.SafeFormatAndMount
}

type photonControllerPersistentDiskMounter struct {
	*photonPersistentDisk
	// Filesystem type, optional.
	fsType string
	// Specifies whether the disk will be attached as read-only.
	readOnly bool
	// diskMounter provides the interface that is used to mount the actual block device.
	diskMounter *mount.SafeFormatAndMount
}

var _ volume.Mounter = &photonControllerPersistentDiskMounter{}

func (b *photonControllerPersistentDiskMounter) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        b.readOnly,
		Managed:         !b.readOnly,
		SupportsSELinux: true,
	}
}

// SetUp attaches the disk and bind mounts to the volume path.
func (b *photonControllerPersistentDiskMounter) SetUp(fsGroup *int64) error {
	return b.SetUpAt(b.GetPath(), fsGroup)
}

// SetUpAt attaches the disk and bind mounts to the volume path.
func (b *photonControllerPersistentDiskMounter) SetUpAt(dir string, fsGroup *int64) error {
	// TODO: handle failed mounts here.
	notMnt, err := b.mounter.IsLikelyNotMountPoint(dir)
	glog.V(1).Infof("PersistentDisk set up: %s %v %v", dir, !notMnt, err)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !notMnt {
		return nil
	}

	globalPDPath := makeGlobalPDName(b.plugin.host, b.diskID)
	if err := b.manager.AttachAndMountDisk(b, globalPDPath); err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		// TODO: we should really eject the attach/detach out into its own control loop.
		detachDiskLogError(b.photonPersistentDisk)
		return err
	}

	// Perform a bind mount to the full path to allow duplicate mounts of the same PD.
	options := []string{"bind"}
	if b.readOnly {
		options = append(options, "ro")
	}
	err = b.mounter.Mount(globalPDPath, dir, "", options)
	if err != nil {
		notMnt, mntErr := b.mounter.IsLikelyNotMountPoint(dir)
		if mntErr != nil {
			glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
			return err
		}
		if !notMnt {
			if mntErr = b.mounter.Unmount(dir); mntErr != nil {
				glog.Errorf("Failed to unmount: %v", mntErr)
				return err
			}
			notMnt, mntErr := b.mounter.IsLikelyNotMountPoint(dir)
			if mntErr != nil {
				glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
				return err
			}
			if !notMnt {
				// This is very odd, we don't expect it.  We'll try again next sync loop.
				glog.Errorf("%s is still mounted, despite call to unmount().  Will try again next sync loop.", dir)
				return err
			}
		}
		os.Remove(dir)
		// TODO: we should really eject the attach/detach out into its own control loop.
		detachDiskLogError(b.photonPersistentDisk)
		return err
	}

	if !b.readOnly {
		volume.SetVolumeOwnership(b, fsGroup)
	}

	return nil
}

func makeGlobalPDName(host volume.VolumeHost, devName string) string {
	return path.Join(host.GetPluginDir(photonControllerPersistentDiskPluginName), "mounts", devName)
}

func (disk *photonPersistentDisk) GetPath() string {
	name := photonControllerPersistentDiskPluginName
	return disk.plugin.host.GetPodVolumeDir(disk.podUID, strings.EscapeQualifiedNameForDisk(name), disk.volName)
}

type photonControllerPersistentDiskUnmounter struct {
	*photonPersistentDisk
}

var _ volume.Unmounter = &photonControllerPersistentDiskUnmounter{}

// Unmounts the bind mount, and detaches the disk only if the PD
// resource was the last reference to that disk on the kubelet.
func (c *photonControllerPersistentDiskUnmounter) TearDown() error {
	return c.TearDownAt(c.GetPath())
}

// Unmounts the bind mount, and detaches the disk only if the PD
// resource was the last reference to that disk on the kubelet.
func (c *photonControllerPersistentDiskUnmounter) TearDownAt(dir string) error {
	notMnt, err := c.mounter.IsLikelyNotMountPoint(dir)
	if err != nil {
		glog.V(1).Info("Error checking if mountpoint ", dir, ": ", err)
		return err
	}
	if notMnt {
		glog.V(1).Info("Not mountpoint, deleting")
		return os.Remove(dir)
	}

	refs, err := mount.GetMountRefs(c.mounter, dir)
	if err != nil {
		glog.V(1).Info("Error getting mountrefs for ", dir, ": ", err)
		return err
	}
	if len(refs) == 0 {
		glog.Warning("Did not find pod-mount for ", dir, " during tear-down")
	}
	// Unmount the bind-mount inside this pod
	if err := c.mounter.Unmount(dir); err != nil {
		glog.V(1).Info("Error unmounting dir ", dir, ": ", err)
		return err
	}
	// If len(refs) is 1, then all bind mounts have been removed, and the
	// remaining reference is the global mount. It is safe to detach.
	if len(refs) == 1 {
		// c.diskID is not initially set for volume-unmounters, so set it here.
		c.diskID = path.Base(refs[0])
		if err := c.manager.DetachDisk(c); err != nil {
			return err
		}
	} else {
		glog.V(1).Infof("Found multiple refs; won't detach photoncontroller disk: %v", refs)
	}
	notMnt, mntErr := c.mounter.IsLikelyNotMountPoint(dir)
	if mntErr != nil {
		glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
		return err
	}
	if notMnt {
		if err := os.Remove(dir); err != nil {
			glog.V(1).Info("Error removing mountpoint ", dir, ": ", err)
			return err
		}
	}
	return nil
}

type photonControllerPersistentDiskDeleter struct {
	*photonPersistentDisk
}

var _ volume.Deleter = &photonControllerPersistentDiskDeleter{}

func (d *photonControllerPersistentDiskDeleter) GetPath() string {
	name := photonControllerPersistentDiskPluginName
	return d.plugin.host.GetPodVolumeDir(d.podUID, strings.EscapeQualifiedNameForDisk(name), d.volName)
}

func (d *photonControllerPersistentDiskDeleter) Delete() error {
	return d.manager.DeleteDisk(d)
}

type photonControllerPersistentDiskProvisioner struct {
	*photonPersistentDisk
	options   volume.VolumeOptions
	namespace string
}

var _ volume.Provisioner = &photonControllerPersistentDiskProvisioner{}

func (c *photonControllerPersistentDiskProvisioner) Provision(pv *api.PersistentVolume) error {
	diskID, sizeGB, err := c.manager.CreateDisk(c, pv)
	if err != nil {
		return err
	}

	pv.Spec.PersistentVolumeSource.PhotonControllerDisk.DiskID = diskID
	pv.Spec.Capacity = api.ResourceList{
		api.ResourceName(api.ResourceStorage): resource.MustParse(fmt.Sprintf("%dGi", sizeGB)),
	}
	return nil
}

func (c *photonControllerPersistentDiskProvisioner) NewPersistentVolumeTemplate() (*api.PersistentVolume, error) {
	// Provide dummy api.PersistentVolume.Spec, it will be filled in
	// photonControllerPersistentDiskProvisioner.Provision()
	return &api.PersistentVolume{
		ObjectMeta: api.ObjectMeta{
			GenerateName: "pv-photoncontroller-",
			Labels:       map[string]string{},
			Annotations: map[string]string{
				"kubernetes.io/createdby": "photoncontroller-pd-dynamic-provisioner",
			},
		},
		Spec: api.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: c.options.PersistentVolumeReclaimPolicy,
			AccessModes:                   c.options.AccessModes,
			Capacity: api.ResourceList{
				api.ResourceName(api.ResourceStorage): c.options.Capacity,
			},
			PersistentVolumeSource: api.PersistentVolumeSource{
				PhotonControllerDisk: &api.PhotonControllerPersistentDiskSource{
					DiskID:    volume.ProvisionedVolumeName,
					FSType:    "ext4",
					Partition: 0,
					ReadOnly:  false,
				},
			},
		},
	}, nil
}
