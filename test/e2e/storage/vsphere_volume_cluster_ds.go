package storage

import (
	"fmt"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib"
	"k8s.io/kubernetes/test/e2e/framework"
)

/*
	Tests to verify volume provisioning on a clustered datastore
	1. Static provisioning
	2. Dynamic provisioning
	3. Dynamic provisioning with spbm policy

	This test reads env
	1. CLUSTER_DATASTORE which should be set to clustered datastore
	2. VSPHERE_SPBM_POLICY_DS_CLUSTER which should be set to a tag based spbm policy tagged to a clustered datastore
*/
var _ = SIGDescribe("Volume Provision On Clustered Datastore", func() {
	f := framework.NewDefaultFramework("volume-provision")

	var client clientset.Interface
	var namespace string
	var scParameters map[string]string
	var clusterDatastore string
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		scParameters = make(map[string]string)

		clusterDatastore = os.Getenv("CLUSTER_DATASTORE")
		Expect(clusterDatastore).NotTo(BeEmpty(), "Please set CLUSTER_DATASTORE system environment")
	})

	/*
		Steps:
		1. Create volume options with datastore to be a clustered datastore
		2. Create a vsphere volume
		3. Create podspec with volume path. Create a corresponding pod
		4. Verify disk is attached
		5. Delete the pod and wait for the disk to be detached
		6. Delete the volume
	*/

	It("verify static provisioning on clustered datastore", func() {
		var volumePath string
		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())

		By("creating a test vsphere volume")
		volumeOptions := new(vclib.VolumeOptions)
		volumeOptions.CapacityKB = 2097152
		volumeOptions.Name = "e2e-vmdk-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		volumeOptions.Datastore = clusterDatastore

		volumePath, err = createVSphereVolume(vsp, volumeOptions)
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By("Deleting the vsphere volume")
			vsp.DeleteVolume(volumePath)
		}()

		podspec := getVSpherePodSpecWithVolumePaths([]string{volumePath}, nil, nil)

		By("Creating pod")
		pod, err := client.CoreV1().Pods(namespace).Create(podspec)
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for pod to be ready")
		Expect(framework.WaitForPodNameRunningInNamespace(client, pod.Name, namespace)).To(Succeed())

		// get fresh pod info
		pod, err = client.CoreV1().Pods(namespace).Get(pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		nodeName := types.NodeName(pod.Spec.NodeName)

		By("Verifying volume is attached")
		isAttached, err := verifyVSphereDiskAttached(vsp, volumePath, types.NodeName(nodeName))
		Expect(err).NotTo(HaveOccurred())
		Expect(isAttached).To(BeTrue(), fmt.Sprintf("disk: %s is not attached with the node", volumePath))

		By("Deleting pod")
		framework.DeletePodWithWait(f, client, pod)

		By("Waiting for volumes to be detached from the node")
		waitForVSphereDiskToDetach(vsp, volumePath, types.NodeName(nodeName))
	})

	/*
		Steps:
		1. Create storage class parameter and specify datastore to be a clustered datastore name
		2. invokeValidPolicyTest - util to do e2e dynamic provision test
	*/
	It("verify dynamic provision with default parameter on clustered datastore", func() {
		scParameters[Datastore] = clusterDatastore
		invokeValidPolicyTest(f, client, namespace, scParameters)
	})

	/*
		Steps:
		1. Create storage class parameter and specify storage policy to be a tag based spbm policy
		2. invokeValidPolicyTest - util to do e2e dynamic provision test
	*/
	It("verify dynamic provision with spbm policy on clustered datastore", func() {
		storagePolicy := os.Getenv("VSPHERE_SPBM_POLICY_DS_CLUSTER")
		Expect(storagePolicy).NotTo(BeEmpty(), "Please set VSPHERE_SPBM_POLICY_DS_CLUSTER system environment")
		scParameters[SpbmStoragePolicy] = storagePolicy
		invokeValidPolicyTest(f, client, namespace, scParameters)
	})
})
