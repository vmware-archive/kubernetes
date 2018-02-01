/*
Copyright 2018 The Kubernetes Authors.

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
	"gopkg.in/gcfg.v1"
	"io"
	"k8s.io/kubernetes/test/e2e/framework"
	"os"
)

const (
	vSphereConfFileEnvVar = "VSPHERE_CONF_FILE"
)

var (
	confFileLocation = os.Getenv(vSphereConfFileEnvVar)
)

// Config represents vSphere configuration
type Config struct {
	Username          string
	Password          string
	Hostname          string
	Port              string
	Datacenters       string
	RoundTripperCount uint
}

// ConfigFile represents the content of vsphere.conf file.
// Users specify the configuration of one or more vSphere instances in vsphere.conf where
// the Kubernetes master and worker nodes are running.
type ConfigFile struct {
	Global struct {
		// vCenter username.
		User string `gcfg:"user"`
		// vCenter password in clear text.
		Password string `gcfg:"password"`
		// Deprecated. Use VirtualCenter to specify multiple vCenter Servers.
		// vCenter IP.
		VCenterIP string `gcfg:"server"`
		// vCenter port.
		VCenterPort string `gcfg:"port"`
		// True if vCenter uses self-signed cert.
		InsecureFlag bool `gcfg:"insecure-flag"`
		// Datacenter in which VMs are located.
		// Deprecated. Use "datacenters" instead.
		Datacenter string `gcfg:"datacenter"`
		// Datacenter in which VMs are located.
		Datacenters string `gcfg:"datacenters"`
		// Datastore in which vmdks are stored.
		// Deprecated. See Workspace.DefaultDatastore
		DefaultDatastore string `gcfg:"datastore"`
		// WorkingDir is path where VMs can be found. Also used to create dummy VMs.
		// Deprecated.
		WorkingDir string `gcfg:"working-dir"`
		// Soap round tripper count (retries = RoundTripper - 1)
		RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
		// Deprecated as the virtual machines will be automatically discovered.
		// VMUUID is the VM Instance UUID of virtual machine which can be retrieved from instanceUuid
		// property in VmConfigInfo, or also set as vc.uuid in VMX file.
		// If not set, will be fetched from the machine via sysfs (requires root)
		VMUUID string `gcfg:"vm-uuid"`
		// Deprecated as virtual machine will be automatically discovered.
		// VMName is the VM name of virtual machine
		// Combining the WorkingDir and VMName can form a unique InstanceID.
		// When vm-name is set, no username/password is required on worker nodes.
		VMName string `gcfg:"vm-name"`
	}

	VirtualCenter map[string]*Config

	Network struct {
		// PublicNetwork is name of the network the VMs are joined to.
		PublicNetwork string `gcfg:"public-network"`
	}

	Disk struct {
		// SCSIControllerType defines SCSI controller to be used.
		SCSIControllerType string `dcfg:"scsicontrollertype"`
	}

	// Endpoint used to create volumes
	Workspace struct {
		VCenterIP        string `gcfg:"server"`
		Datacenter       string `gcfg:"datacenter"`
		Folder           string `gcfg:"folder"`
		DefaultDatastore string `gcfg:"default-datastore"`
		ResourcePoolPath string `gcfg:"resourcepool-path"`
	}
}

// GetVSphereInstances parses vsphere.conf and returns VSphere instances
func GetVSphereInstances() (map[string]*VSphere, error) {
	cfg, err := getConfig()
	if err != nil {
		return nil, err
	}
	return populateInstanceMap(cfg)
}

func getConfig() (*ConfigFile, error) {
	if confFileLocation == "" {
		return nil, fmt.Errorf("Env variable 'VSPHERE_CONF_FILE' is not set.")
	}
	confFile, err := os.Open(confFileLocation)
	if err != nil {
		return nil, err
	}
	defer confFile.Close()
	cfg, err := readConfig(confFile)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// readConfig parses vSphere cloud config file into ConfigFile.
func readConfig(config io.Reader) (ConfigFile, error) {
	if config == nil {
		err := fmt.Errorf("no vSphere cloud provider config file given")
		return ConfigFile{}, err
	}

	var cfg ConfigFile
	err := gcfg.ReadInto(&cfg, config)
	return cfg, err
}

func populateInstanceMap(cfg *ConfigFile) (map[string]*VSphere, error) {
	vsphereInstances := make(map[string]*VSphere)

	// Check if the vsphere.conf is in old format. In this
	// format the cfg.VirtualCenter will be nil or empty.
	if cfg.VirtualCenter == nil || len(cfg.VirtualCenter) == 0 {
		framework.Logf("Config is not per vSphere and is in old format")
		if cfg.Global.User == "" {
			framework.Logf("Global.User is empty!")
			return nil, errors.New("Global.User is empty!")
		}
		if cfg.Global.Password == "" {
			framework.Logf("Global.Password is empty!")
			return nil, errors.New("Global.Password is empty!")
		}
		if cfg.Global.WorkingDir == "" {
			framework.Logf("Global.WorkingDir is empty!")
			return nil, errors.New("Global.WorkingDir is empty!")
		}
		if cfg.Global.VCenterIP == "" {
			framework.Logf("Global.VCenterIP is empty!")
			return nil, errors.New("Global.VCenterIP is empty!")
		}
		if cfg.Global.Datacenter == "" {
			framework.Logf("Global.Datacenter is empty!")
			return nil, errors.New("Global.Datacenter is empty!")
		}
		cfg.Workspace.VCenterIP = cfg.Global.VCenterIP
		cfg.Workspace.Datacenter = cfg.Global.Datacenter
		cfg.Workspace.Folder = cfg.Global.WorkingDir
		cfg.Workspace.DefaultDatastore = cfg.Global.DefaultDatastore

		vcConfig := Config{
			Username:          cfg.Global.User,
			Password:          cfg.Global.Password,
			Hostname:          cfg.Global.VCenterIP,
			Port:              cfg.Global.VCenterPort,
			Datacenters:       cfg.Global.Datacenter,
			RoundTripperCount: cfg.Global.RoundTripperCount,
		}
		vsphereIns := VSphere{
			Config: &vcConfig,
		}
		vsphereInstances[cfg.Global.VCenterIP] = &vsphereIns
	} else {
		if cfg.Workspace.VCenterIP == "" || cfg.Workspace.Folder == "" || cfg.Workspace.Datacenter == "" {
			msg := fmt.Sprintf("All fields in workspace are mandatory."+
				" vsphere.conf does not have the workspace specified correctly. cfg.Workspace: %+v", cfg.Workspace)
			framework.Logf(msg)
			return nil, errors.New(msg)
		}
		for vcServer, vcConfig := range cfg.VirtualCenter {
			framework.Logf("Initializing vc server %s", vcServer)
			if vcServer == "" {
				framework.Logf("vsphere.conf does not have the VirtualCenter IP address specified")
				return nil, errors.New("vsphere.conf does not have the VirtualCenter IP address specified")
			}
			vcConfig.Hostname = vcServer

			if vcConfig.Username == "" {
				vcConfig.Username = cfg.Global.User
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cfg.Global.Password
			}
			if vcConfig.Username == "" {
				msg := fmt.Sprintf("vcConfig.User is empty for vc %s!", vcServer)
				framework.Logf(msg)
				return nil, errors.New(msg)
			}
			if vcConfig.Password == "" {
				msg := fmt.Sprintf("vcConfig.Password is empty for vc %s!", vcServer)
				framework.Logf(msg)
				return nil, errors.New(msg)
			}
			if vcConfig.Port == "" {
				vcConfig.Port = cfg.Global.VCenterPort
			}
			if vcConfig.Datacenters == "" {
				if cfg.Global.Datacenters != "" {
					vcConfig.Datacenters = cfg.Global.Datacenters
				} else {
					// cfg.Global.Datacenter is deprecated, so giving it the last preference.
					vcConfig.Datacenters = cfg.Global.Datacenter
				}
			}
			if vcConfig.RoundTripperCount == 0 {
				vcConfig.RoundTripperCount = cfg.Global.RoundTripperCount
			}

			vsphereIns := VSphere{
				Config: vcConfig,
			}
			vsphereInstances[vcServer] = &vsphereIns
		}
	}

	framework.Logf("ConfigFile %v \n vSphere instances %v", cfg, vsphereInstances)
	return vsphereInstances, nil
}
