package vsphere

import (
	"net/url"

	"k8s.io/kubernetes/pkg/cloudprovider"
)

// ProviderName for this vsphere cloud provider
const ProviderName = "vsphere"

// TagNameKubernetesCluster the tag name we use to differentiate multiple logically independent clusters running in the same AZ
const TagNameKubernetesCluster = "KubernetesCluster"

// VSphere the cloud provider interface implementation
type VSphere struct {
	BaseURL    *url.URL
	Datacenter string
	Datastore  string
	Cluster    string
	Insecure   bool
}

// LoadBalancer gets a load balancer, this is implemented as a distributed vswitch
func (v *VSphere) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances gets the compute resources in a vsphere cluster
func (v *VSphere) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// Zones gets the zones in a vsphere cluster
func (v *VSphere) Zones() (cloudprovider.Zones, bool) {
	return v, true
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
func (v *VSphere) GetZone() (cloudprovider.Zone, error) {
	return cloudprovider.Zone{
		FailureDomain: v.Cluster,
		Region:        v.Datacenter,
	}, nil
}

// Clusters returns a clusters interface in a vsphere cluster
func (v *VSphere) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns the routes interface for a vsphere cluster
func (v *VSphere) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider id
func (v *VSphere) ProviderName() string {
	return ProviderName
}

// ScrubDNS configures a dns server for a vsphere cluster
func (v *VSphere) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return
}

type vsphereInstances struct {
}
