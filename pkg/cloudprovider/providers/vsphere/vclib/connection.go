package vclib

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/golang/glog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"golang.org/x/net/context"
)

type VSphereConnection struct {
	GoVmomiClient     *govmomi.Client
	Username          string
	Password          string
	Hostname          string
	Port              string
	Insecure          bool
	RoundTripperCount uint
}

var (
	vsphereConnection *VSphereConnection
	clientLock        sync.Mutex
)

// Function to make connection to vCenter Server
// After successful connection, Client will set to govmomi client object.
func (connection VSphereConnection) Connect() error {
	var err error
	clientLock.Lock()
	defer clientLock.Unlock()

	if connection.GoVmomiClient == nil {
		connection.GoVmomiClient, err = connection.newClient()
		if err != nil {
			glog.Errorf("Failed to create govmomi client. err: %v", err)
			return err
		}
		return nil
	}
	m := session.NewManager(connection.GoVmomiClient.Client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userSession, err := m.UserSession(ctx)
	if err != nil {
		glog.Errorf("Error while obtaining user session. err: %v", err)
		return err
	}
	if userSession != nil {
		return nil
	}

	glog.Warningf("Creating new client session since the existing session is not valid or not authenticated")
	connection.GoVmomiClient.Logout(ctx)
	connection.GoVmomiClient, err = connection.newClient()
	if err != nil {
		glog.Errorf("Failed to create govmomi client. err: %v", err)
		return err
	}
	return nil
}

// Function to obtain DataCenter Object from given DataCenter name
func (connection VSphereConnection) GetDataCenter(ctx context.Context, datacenterName string) (DataCenter, error) {
	finder := find.NewFinder(vsphereConnection.GoVmomiClient.Client, true)
	dataCenter, err := finder.Datacenter(ctx, datacenterName)
	if err != nil {
		glog.Errorf("Failed to find the data center: %s. err: %v", datacenterName, err)
		return nil, err
	}
	dc := DataCenter{dataCenter}
	return dc, nil
}

// Private function to create client object, called from VSphereConnection. Connect()
func (connection VSphereConnection) newClient() (*govmomi.Client, error) {
	url, err := url.Parse(fmt.Sprintf("https://%s/sdk", connection.Hostname))
	if err != nil {
		glog.Errorf("Failed to parse URL: %s. err: %v", url, err)
		return nil, err
	}
	url.User = url.UserPassword(connection.Username, connection.Password)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := govmomi.NewClient(ctx, url, connection.Insecure)
	if err != nil {
		glog.Errorf("Failed to create new client. err: %v", err)
		return nil, err
	}
	if connection.RoundTripperCount == 0 {
		connection.RoundTripperCount = RoundTripperDefaultCount
	}
	client.RoundTripper = vim25.Retry(client.RoundTripper, vim25.TemporaryNetworkError(int(connection.RoundTripperCount)))
	return client, nil
}
