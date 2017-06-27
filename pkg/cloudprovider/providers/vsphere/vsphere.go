/*
Copyright 2016 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/gcfg.v1"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/v1"
	v1helper "k8s.io/kubernetes/pkg/api/v1/helper"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib/diskmanagers"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	ProviderName                  = "vsphere"
	ActivePowerState              = "poweredOn"
	VolDir                        = "kubevols"
	RoundTripperDefaultCount      = 3
	DummyVMPrefixName             = "vsphere-k8s"
	VSANDatastoreType             = "vsan"
	MacOuiVC                      = "00:50:56"
	MacOuiEsx                     = "00:0c:29"
	CleanUpDummyVMRoutineInterval = 5
	UUIDPath                      = "/sys/class/dmi/id/product_serial"
	UUIDPrefix                    = "VMware-"
)

var cleanUpRoutineInitialized = false

var clientLock sync.Mutex
var cleanUpRoutineInitLock sync.Mutex
var cleanUpDummyVMLock sync.RWMutex

// VSphere is an implementation of cloud provider Interface for VSphere.
type VSphere struct {
	conn *vclib.VSphereConnection
	cfg  *VSphereConfig
	// InstanceID of the server where this VSphere object is instantiated.
	localInstanceID string
}

// VSphereConfig information that is used by vSphere Cloud Provider to connect to VC
type VSphereConfig struct {
	Global struct {
		// vCenter username.
		User string `gcfg:"user"`
		// vCenter password in clear text.
		Password string `gcfg:"password"`
		// vCenter IP.
		VCenterIP string `gcfg:"server"`
		// vCenter port.
		VCenterPort string `gcfg:"port"`
		// True if vCenter uses self-signed cert.
		InsecureFlag bool `gcfg:"insecure-flag"`
		// Datacenter in which VMs are located.
		Datacenter string `gcfg:"datacenter"`
		// Datastore in which vmdks are stored.
		Datastore string `gcfg:"datastore"`
		// WorkingDir is path where VMs can be found.
		WorkingDir string `gcfg:"working-dir"`
		// Soap round tripper count (retries = RoundTripper - 1)
		RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
		// VMUUID is the VM Instance UUID of virtual machine which can be retrieved from instanceUuid
		// property in VmConfigInfo, or also set as vc.uuid in VMX file.
		// If not set, will be fetched from the machine via sysfs (requires root)
		VMUUID string `gcfg:"vm-uuid"`
		// VMName is the VM name of virtual machine
		// Combining the WorkingDir and VMName can form a unique InstanceID.
		// When vm-name is set, no username/password is required on worker nodes.
		VMName string `gcfg:"vm-name"`
	}

	Network struct {
		// PublicNetwork is name of the network the VMs are joined to.
		PublicNetwork string `gcfg:"public-network"`
	}

	Disk struct {
		// SCSIControllerType defines SCSI controller to be used.
		SCSIControllerType string `dcfg:"scsicontrollertype"`
	}
}

type Volumes interface {
	// AttachDisk attaches given disk to given node. Current node
	// is used when nodeName is empty string.
	AttachDisk(vmDiskPath string, storagePolicyID string, nodeName k8stypes.NodeName) (diskID string, diskUUID string, err error)

	// DetachDisk detaches given disk to given node. Current node
	// is used when nodeName is empty string.
	// Assumption: If node doesn't exist, disk is already detached from node.
	DetachDisk(volPath string, nodeName k8stypes.NodeName) error

	// DiskIsAttached checks if a disk is attached to the given node.
	// Assumption: If node doesn't exist, disk is not attached to the node.
	DiskIsAttached(volPath string, nodeName k8stypes.NodeName) (bool, error)

	// DisksAreAttached checks if a list disks are attached to the given node.
	// Assumption: If node doesn't exist, disks are not attached to the node.
	DisksAreAttached(volPath []string, nodeName k8stypes.NodeName) (map[string]bool, error)

	// CreateVolume creates a new vmdk with specified parameters.
	CreateVolume(volumeOptions *vclib.VolumeOptions) (volumePath string, err error)

	// DeleteVolume deletes vmdk.
	DeleteVolume(vmDiskPath string) error
}

// Parses vSphere cloud config file and stores it into VSphereConfig.
func readConfig(config io.Reader) (VSphereConfig, error) {
	if config == nil {
		err := fmt.Errorf("no vSphere cloud provider config file given")
		return VSphereConfig{}, err
	}

	var cfg VSphereConfig
	err := gcfg.ReadInto(&cfg, config)
	return cfg, err
}

func init() {
	registerMetrics()
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return newVSphere(cfg)
	})
}

// Initialize passes a Kubernetes clientBuilder interface to the cloud provider
func (vs *VSphere) Initialize(clientBuilder controller.ControllerClientBuilder) {}

func newVSphere(cfg VSphereConfig) (*VSphere, error) {
	var err error
	if cfg.Disk.SCSIControllerType == "" {
		cfg.Disk.SCSIControllerType = vclib.PVSCSIControllerType
	} else if !vclib.CheckControllerSupported(cfg.Disk.SCSIControllerType) {
		glog.Errorf("%v is not a supported SCSI Controller type. Please configure 'lsilogic-sas' OR 'pvscsi'", cfg.Disk.SCSIControllerType)
		return nil, errors.New("Controller type not supported. Please configure 'lsilogic-sas' OR 'pvscsi'")
	}
	if cfg.Global.WorkingDir != "" {
		cfg.Global.WorkingDir = path.Clean(cfg.Global.WorkingDir)
	}
	if cfg.Global.RoundTripperCount == 0 {
		cfg.Global.RoundTripperCount = RoundTripperDefaultCount
	}
	if cfg.Global.VCenterPort != "" {
		glog.Warningf("port is a deprecated field in vsphere.conf and will be removed in future release.")
	}
	if cfg.Global.VMUUID == "" {
		// This needs root privileges on the host, and will fail otherwise.
		cfg.Global.VMUUID, err = getvmUUID()
		if err != nil {
			glog.Errorf("Failed to get VM UUID. err: %+v", err)
			return nil, err
		}
	}
	vSphereConn := vclib.VSphereConnection{
		Username:          cfg.Global.User,
		Password:          cfg.Global.Password,
		Hostname:          cfg.Global.VCenterIP,
		Insecure:          cfg.Global.InsecureFlag,
		RoundTripperCount: cfg.Global.RoundTripperCount,
	}
	var instanceID string
	if cfg.Global.VMName == "" {
		// if VMName is not set in the cloud config file, each nodes (including worker nodes) need credentials to obtain VMName from vCenter
		glog.V(4).Infof("Cannot find VMName from cloud config file, start obtaining it from vCenter")
		err = vSphereConn.Connect()
		if err != nil {
			glog.Errorf("Failed to connect to vSphere")
			return nil, err
		}
		// Create context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		dc, err := vclib.GetDatacenter(ctx, &vSphereConn, cfg.Global.Datacenter)
		if err != nil {
			return nil, err
		}
		vm, err := dc.GetVMByUUID(ctx, cfg.Global.VMUUID)
		if err != nil {
			return nil, err
		}
		vmName, err := vm.ObjectName(ctx)
		if err != nil {
			return nil, err
		}
		instanceID = vmName
	} else {
		instanceID = cfg.Global.VMName
	}
	vs := VSphere{
		conn:            &vSphereConn,
		cfg:             &cfg,
		localInstanceID: instanceID,
	}
	runtime.SetFinalizer(&vs, logout)
	return &vs, nil
}

func logout(vs *VSphere) {
	if vs.conn.GoVmomiClient != nil {
		vs.conn.GoVmomiClient.Logout(context.TODO())
	}
}

// Instances returns an implementation of Instances for vSphere.
func (vs *VSphere) Instances() (cloudprovider.Instances, bool) {
	return vs, true
}

func getLocalIP() ([]v1.NodeAddress, error) {
	addrs := []v1.NodeAddress{}
	ifaces, err := net.Interfaces()
	if err != nil {
		glog.Errorf("net.Interfaces() failed for NodeAddresses - %v", err)
		return nil, err
	}
	for _, i := range ifaces {
		localAddrs, err := i.Addrs()
		if err != nil {
			glog.Warningf("Failed to extract addresses for NodeAddresses - %v", err)
		} else {
			for _, addr := range localAddrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						// Filter external IP by MAC address OUIs from vCenter and from ESX
						var addressType v1.NodeAddressType
						if strings.HasPrefix(i.HardwareAddr.String(), MacOuiVC) ||
							strings.HasPrefix(i.HardwareAddr.String(), MacOuiEsx) {
							v1helper.AddToNodeAddresses(&addrs,
								v1.NodeAddress{
									Type:    v1.NodeExternalIP,
									Address: ipnet.IP.String(),
								},
								v1.NodeAddress{
									Type:    v1.NodeInternalIP,
									Address: ipnet.IP.String(),
								},
							)
						}
						glog.V(4).Infof("Find local IP address %v and set type to %v", ipnet.IP.String(), addressType)
					}
				}
			}
		}
	}
	return addrs, nil
}

// Get the VM Managed Object instance by from the node
func (vs *VSphere) getVMByName(ctx context.Context, nodeName k8stypes.NodeName) (*vclib.VirtualMachine, error) {
	// Ensure client is logged in and session is valid
	err := vs.conn.Connect()
	if err != nil {
		return nil, err
	}
	dc, err := vclib.GetDatacenter(ctx, vs.conn, vs.cfg.Global.Datacenter)
	if err != nil {
		return nil, err
	}
	vmPath := vs.cfg.Global.WorkingDir + "/" + nodeNameToVMName(nodeName)
	vm, err := dc.GetVMByPath(ctx, vmPath)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// NodeAddresses is an implementation of Instances.NodeAddresses.
func (vs *VSphere) NodeAddresses(nodeName k8stypes.NodeName) ([]v1.NodeAddress, error) {
	// Get local IP addresses if node is local node
	if vs.localInstanceID == nodeNameToVMName(nodeName) {
		return getLocalIP()
	}
	addrs := []v1.NodeAddress{}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm, err := vs.getVMByName(ctx, nodeName)
	if err != nil {
		glog.Errorf("Failed to get VM object for node: %q. err: +%v", nodeNameToVMName(nodeName), err)
		return nil, err
	}
	vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*vclib.VirtualMachine{vm}, []string{"guest.net"})
	if err != nil {
		return nil, err
	}
	// retrieve VM's ip(s)
	for _, v := range vmMoList[0].Guest.Net {
		if vs.cfg.Network.PublicNetwork == v.Network {
			for _, ip := range v.IpAddress {
				if net.ParseIP(ip).To4() != nil {
					v1helper.AddToNodeAddresses(&addrs,
						v1.NodeAddress{
							Type:    v1.NodeExternalIP,
							Address: ip,
						}, v1.NodeAddress{
							Type:    v1.NodeInternalIP,
							Address: ip,
						},
					)
				}
			}
		}
	}
	return addrs, nil
}

// NodeAddressesByProviderID returns the node addresses of an instances with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
func (vs *VSphere) NodeAddressesByProviderID(providerID string) ([]v1.NodeAddress, error) {
	return []v1.NodeAddress{}, errors.New("unimplemented")
}

// AddSSHKeyToAllInstances add SSH key to all instances
func (vs *VSphere) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("unimplemented")
}

// CurrentNodeName gives the current node name
func (vs *VSphere) CurrentNodeName(hostname string) (k8stypes.NodeName, error) {
	return vmNameToNodeName(vs.localInstanceID), nil
}

// nodeNameToVMName maps a NodeName to the vmware infrastructure name
func nodeNameToVMName(nodeName k8stypes.NodeName) string {
	return string(nodeName)
}

// nodeNameToVMName maps a vmware infrastructure name to a NodeName
func vmNameToNodeName(vmName string) k8stypes.NodeName {
	return k8stypes.NodeName(vmName)
}

// ExternalID returns the cloud provider ID of the node with the specified Name (deprecated).
func (vs *VSphere) ExternalID(nodeName k8stypes.NodeName) (string, error) {
	return vs.InstanceID(nodeName)
}

// InstanceID returns the cloud provider ID of the node with the specified Name.
func (vs *VSphere) InstanceID(nodeName k8stypes.NodeName) (string, error) {
	if vs.localInstanceID == nodeNameToVMName(nodeName) {
		return vs.cfg.Global.WorkingDir + "/" + vs.localInstanceID, nil
	}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm, err := vs.getVMByName(ctx, nodeName)
	if err != nil {
		glog.Errorf("Failed to get VM object for node: %q. err: +%v", nodeNameToVMName(nodeName), err)
		return "", err
	}
	vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*vclib.VirtualMachine{vm}, []string{"summary"})
	if err != nil {
		return "", err
	}
	if vmMoList[0].Summary.Runtime.PowerState == ActivePowerState {
		return "/" + vm.InventoryPath, nil
	}
	if vmMoList[0].Summary.Config.Template == false {
		glog.Warningf("VM %s, is not in %s state", nodeName, ActivePowerState)
	} else {
		glog.Warningf("VM %s, is a template", nodeName)
	}
	return "", cloudprovider.InstanceNotFound
}

// InstanceTypeByProviderID returns the cloudprovider instance type of the node with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
func (vs *VSphere) InstanceTypeByProviderID(providerID string) (string, error) {
	return "", errors.New("unimplemented")
}

func (vs *VSphere) InstanceType(name k8stypes.NodeName) (string, error) {
	return "", nil
}

func (vs *VSphere) Clusters() (cloudprovider.Clusters, bool) {
	return nil, true
}

// ProviderName returns the cloud provider ID.
func (vs *VSphere) ProviderName() string {
	return ProviderName
}

// LoadBalancer returns an implementation of LoadBalancer for vSphere.
func (vs *VSphere) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Zones returns an implementation of Zones for Google vSphere.
func (vs *VSphere) Zones() (cloudprovider.Zones, bool) {
	glog.V(1).Info("The vSphere cloud provider does not support zones")
	return nil, false
}

// Routes returns a false since the interface is not supported for vSphere.
func (vs *VSphere) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ScrubDNS filters DNS settings for pods.
func (vs *VSphere) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nameservers, searches
}

// AttachDisk attaches given virtual disk volume to the compute running kubelet.
func (vs *VSphere) AttachDisk(vmDiskPath string, storagePolicyID string, nodeName k8stypes.NodeName) (diskID string, diskUUID string, err error) {
	return "", "", nil
}

// DetachDisk detaches given virtual disk volume from the compute running kubelet.
func (vs *VSphere) DetachDisk(volPath string, nodeName k8stypes.NodeName) error {
	return nil
}

// DiskIsAttached returns if disk is attached to the VM using controllers supported by the plugin.
func (vs *VSphere) DiskIsAttached(volPath string, nodeName k8stypes.NodeName) (bool, error) {
	var vSphereInstance string
	if nodeName == "" {
		vSphereInstance = vs.localInstanceID
		nodeName = vmNameToNodeName(vSphereInstance)
	} else {
		vSphereInstance = nodeNameToVMName(nodeName)
	}
	nodeExist, err := vs.NodeExists(nodeName)
	if err != nil {
		glog.Errorf("Failed to check whether node exist. err: %s.", err)
		return false, err
	}
	if !nodeExist {
		glog.Errorf("DiskIsAttached failed to determine whether disk %q is still attached: node %q does not exist",
			volPath,
			vSphereInstance)
		return false, fmt.Errorf("DiskIsAttached failed to determine whether disk %q is still attached: node %q does not exist",
			volPath,
			vSphereInstance)
	}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm, err := vs.getVMByName(ctx, nodeName)
	if err != nil {
		glog.Errorf("Failed to get VM object for node: %q. err: +%v", nodeNameToVMName(nodeName), err)
		return false, err
	}
	attached, err := vm.IsDiskAttached(ctx, volPath)
	return attached, err
}

// DisksAreAttached returns if disks are attached to the VM using controllers supported by the plugin.
func (vs *VSphere) DisksAreAttached(volPaths []string, nodeName k8stypes.NodeName) (map[string]bool, error) {
	attached := make(map[string]bool)
	if len(volPaths) == 0 {
		return attached, nil
	}
	var vSphereInstance string
	if nodeName == "" {
		vSphereInstance = vs.localInstanceID
		nodeName = vmNameToNodeName(vSphereInstance)
	} else {
		vSphereInstance = nodeNameToVMName(nodeName)
	}
	nodeExist, err := vs.NodeExists(nodeName)
	if err != nil {
		glog.Errorf("Failed to check whether node exist. err: %s.", err)
		return nil, err
	}
	if !nodeExist {
		glog.Errorf("DisksAreAttached failed to determine whether disks %v are still attached: node %q does not exist",
			volPaths,
			vSphereInstance)
		return nil, fmt.Errorf("DisksAreAttached failed to determine whether disks %v are still attached: node %q does not exist",
			volPaths,
			vSphereInstance)
	}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm, err := vs.getVMByName(ctx, nodeName)
	if err != nil {
		glog.Errorf("Failed to get VM object for node: %q. err: +%v", nodeNameToVMName(nodeName), err)
		return nil, err
	}
	for _, volPath := range volPaths {
		result, err := vm.IsDiskAttached(ctx, volPath)
		if err == nil {
			if result {
				attached[volPath] = true
			} else {
				attached[volPath] = false
			}
		} else {
			return nil, err
		}
	}
	return attached, nil
}

// CreateVolume creates a volume of given size (in KiB).
func (vs *VSphere) CreateVolume(volumeOptions *vclib.VolumeOptions) (volumePath string, err error) {
	var datastore string
	// Default datastore is the datastore in the vSphere config file that is used to initialize vSphere cloud provider.
	if volumeOptions.Datastore == "" {
		datastore = vs.cfg.Global.Datastore
	} else {
		datastore = volumeOptions.Datastore
	}
	// Ensure client is logged in and session is valid
	err = vs.conn.Connect()
	if err != nil {
		return "", err
	}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dc, err := vclib.GetDatacenter(ctx, vs.conn, vs.cfg.Global.Datacenter)
	if err != nil {
		return "", err
	}
	var vmOptions *vclib.VMOptions
	if volumeOptions.VSANStorageProfileData != "" || volumeOptions.StoragePolicyName != "" {
		// Acquire a read lock to ensure multiple PVC requests can be processed simultaneously.
		cleanUpDummyVMLock.RLock()
		defer cleanUpDummyVMLock.RUnlock()
		// Create a new background routine that will delete any dummy VM's that are left stale.
		// This routine will get executed for every 5 minutes and gets initiated only once in its entire lifetime.
		cleanUpRoutineInitLock.Lock()
		if !cleanUpRoutineInitialized {
			go vs.cleanUpDummyVMs(DummyVMPrefixName)
			cleanUpRoutineInitialized = true
		}
		cleanUpRoutineInitLock.Unlock()
		vmOptions, err = vs.setVMOptions(ctx, dc)
		if err != nil {
			return "", err
		}
	}
	if volumeOptions.StoragePolicyName != "" && volumeOptions.Datastore == "" {
		datastore, err = getPbmCompatibleDatastore(ctx, dc.Client(), volumeOptions.StoragePolicyName, vmOptions.VMFolder)
		if err != nil {
			return "", err
		}
	}
	ds, err := dc.GetDatastoreByName(ctx, datastore)
	if err != nil {
		return "", err
	}
	volumeOptions.Datastore = datastore
	kubeVolsPath := filepath.Clean(ds.Path(VolDir)) + "/"
	err = ds.CreateDirectory(ctx, kubeVolsPath, false)
	if err != nil && err != vclib.ErrFileAlreadyExist {
		glog.Errorf("Cannot create dir %#v. err %s", kubeVolsPath, err)
		return "", err
	}
	volumePath = kubeVolsPath + volumeOptions.Name + ".vmdk"
	disk := diskmanagers.VirtualDisk{
		DiskPath:      volumePath,
		VolumeOptions: volumeOptions,
		VMOptions:     vmOptions,
	}
	err = disk.Create(ctx, ds)
	if err != nil {
		return "", err
	}
	return volumePath, nil
}

// DeleteVolume deletes a volume given volume name.
func (vs *VSphere) DeleteVolume(vmDiskPath string) error {
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Ensure client is logged in and session is valid
	err := vs.conn.Connect()
	if err != nil {
		return err
	}
	dc, err := vclib.GetDatacenter(ctx, vs.conn, vs.cfg.Global.Datacenter)
	if err != nil {
		return err
	}
	ds, err := dc.GetDatastoreByName(ctx, vs.cfg.Global.Datastore)
	if err != nil {
		return err
	}
	vmDiskPath = removeClusterFromVDiskPath(vmDiskPath)
	disk := diskmanagers.VirtualDisk{
		DiskPath:      vmDiskPath,
		VolumeOptions: &vclib.VolumeOptions{},
		VMOptions:     &vclib.VMOptions{},
	}
	err = disk.Delete(ctx, ds)
	return err
}

// NodeExists checks if the node with given nodeName exist.
// Returns false if VM doesn't exist or VM is in powerOff state.
func (vs *VSphere) NodeExists(nodeName k8stypes.NodeName) (bool, error) {
	if nodeName == "" {
		return false, nil
	}
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm, err := vs.getVMByName(ctx, nodeName)
	if err != nil {
		glog.Errorf("Failed to get VM object for node: %s. err: +%v", nodeNameToVMName(nodeName), err)
		return false, err
	}
	vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*vclib.VirtualMachine{vm}, []string{"summary"})
	if err != nil {
		return false, err
	}
	if vmMoList[0].Summary.Runtime.PowerState == ActivePowerState {
		return true, nil
	}
	if vmMoList[0].Summary.Config.Template == false {
		glog.Warningf("VM %s, is not in %s state", nodeName, ActivePowerState)
	} else {
		glog.Warningf("VM %s, is a template", nodeName)
	}
	return false, nil
}
