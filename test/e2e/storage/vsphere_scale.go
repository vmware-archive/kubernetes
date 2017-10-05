package storage

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
)

/*
	Induce stress to create volumes in parallel with multiple threads based on user configurable values for number of volumes and threads.
	The following actions will be performed as part of this test.

	1. Create Storage Classes of 4 Categories (Default, SC with Non Default Datastore, SC with SPBM Policy, SC with VSAN Storage Capalibilies.)
    2. Read VCP_SCALE_VOLUME_COUNT from System Environment.
	3. Launch goroutine for creating volumes n times. Here n is the value specified by the user in the system env VCP_STRESS_INSTANCES env variable.
	4. Each instance of routine creates m number of volumes using the storage classes created in step-1. Here m is value specified by the user in the system env VCP_STRESS_VOLUMES_PER_INSTANCE.
*/
const (
	NodeLabelKey = "vsphere_e2e_label"
)

// NodeSelector stores info about node and the node selector
type NodeSelector struct {
	nodeName  string
	nodeLabel map[string]string
}

var _ = SIGDescribe("vcp-at-scale", func() {
	f := framework.NewDefaultFramework("vcp-at-scale")
	var (
		client           clientset.Interface
		namespace        string
		areNodesLabeled  bool
		nodeSelectorList []*NodeSelector
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		Expect(os.Getenv("VCP_SCALE_VOLUME_COUNT")).NotTo(BeEmpty(), "ENV VCP_SCALE_VOLUME_COUNT is not set")
		Expect(os.Getenv("VSPHERE_SPBM_GOLD_POLICY")).NotTo(BeEmpty(), "ENV VSPHERE_SPBM_GOLD_POLICY is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "ENV VSPHERE_DATASTORE is not set")
		if !areNodesLabeled {
			createNodeLabels(client, namespace, nodeSelectorList)
			isNodeLabeled = true
		}
	})

	It("vsphere scale tests", func() {
		numberOfInstances := 4
		// Volumes will be provisioned with each different types of Storage Class
		scArrays := make([]*storageV1.StorageClass, 4)
		// Create default vSphere Storage Class
		By("Creating Storage Class : sc-default")
		scDefaultSpec := getVSphereStorageClassSpec("sc-default", nil)
		scDefault, err := client.StorageV1().StorageClasses().Create(scDefaultSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scDefault.Name, nil)
		scArrays[0] = scDefault

		// Create Storage Class with vsan storage capabilities
		By("Creating Storage Class : sc-vsan")
		var scvsanParameters map[string]string
		scvsanParameters = make(map[string]string)
		scvsanParameters[Policy_HostFailuresToTolerate] = "1"
		scvsanSpec := getVSphereStorageClassSpec("sc-vsan", scvsanParameters)
		scvsan, err := client.StorageV1().StorageClasses().Create(scvsanSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scvsan.Name, nil)
		scArrays[1] = scvsan

		// Create Storage Class with SPBM Policy
		By("Creating Storage Class : sc-spbm")
		var scSpbmPolicyParameters map[string]string
		scSpbmPolicyParameters = make(map[string]string)
		goldPolicy := os.Getenv("VSPHERE_SPBM_GOLD_POLICY")
		Expect(goldPolicy).NotTo(BeEmpty())
		scSpbmPolicyParameters[SpbmStoragePolicy] = goldPolicy
		scSpbmPolicySpec := getVSphereStorageClassSpec("sc-spbm", scSpbmPolicyParameters)
		scSpbmPolicy, err := client.StorageV1().StorageClasses().Create(scSpbmPolicySpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scSpbmPolicy.Name, nil)
		scArrays[2] = scSpbmPolicy

		// Create Storage Class with User Specified Datastore.
		By("Creating Storage Class : sc-user-specified-datastore")
		var scWithDatastoreParameters map[string]string
		scWithDatastoreParameters = make(map[string]string)
		datastore := os.Getenv("VSPHERE_DATASTORE")
		Expect(goldPolicy).NotTo(BeEmpty())
		scWithDatastoreParameters[Datastore] = datastore
		scWithDatastoreSpec := getVSphereStorageClassSpec("sc-user-specified-ds", scWithDatastoreParameters)
		scWithDatastore, err := client.StorageV1().StorageClasses().Create(scWithDatastoreSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scWithDatastore.Name, nil)
		scArrays[3] = scWithDatastore

		volumeCount, err := strconv.Atoi(os.Getenv("VCP_SCALE_VOLUME_COUNT"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_SCALE_VOLUME_COUNT")

		var wg sync.WaitGroup
		wg.Add(numberOfInstances)
		volumeCountPerInstance := volumeCount / numberOfInstances
		for instanceCount := 0; instanceCount < numberOfInstances; instanceCount++ {
			if instanceCount == numberOfInstances-1 {
				volumeCountPerInstance = volumeCount
			}
			volumeCount = volumeCount - volumeCountPerInstance
			go CreateVolumesInParallel(client, namespace, scArrays, volumeCountPerInstance, &wg, nodeSelectorList)
		}
		wg.Wait()
	})
})

// CreateVolumesInParallel ...
func CreateVolumesInParallel(client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumeCountPerInstance int, wg *sync.WaitGroup, nodeSelectorList []*NodeSelector) {
	defer wg.Done()
	for index := 0; index < volumeCountPerInstance; index = index + 2 {
		pvclaims := make([]*v1.PersistentVolumeClaim, 2)
		for i = 0; i < 2; i++ {
			By(fmt.Sprintf("Creating PVC %q using the Storage Class", index+1))
			pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc[index%len(sc)]))
			Expect(err).NotTo(HaveOccurred())
			pvclaims = append(pvclaims, pvclaim)
			defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)
		}

		By("Waiting for claim to be in bound phase")
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims)
		Expect(err).NotTo(HaveOccurred())

		By("Creating pod to attach PV to the node")
		// Create pod to attach Volume to Node
		pod, err := framework.CreatePod(client, namespace, nodeSelectorList[index%len(nodeSelectorList)].nodeLabel, pvclaims, false, "")
		Expect(err).NotTo(HaveOccurred())

		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())
		By("Verify the volume is accessible and available in the pod")
		verifyVSphereVolumesAccessible(pod, persistentvolumes, vsp)

		By("Deleting pod")
		framework.DeletePodWithWait(f, client, pod)

		By("Waiting for volumes to be detached from the node")
		waitForVSphereDiskToDetach(vsp, persistentvolumes[0].Spec.VsphereVolume.VolumePath, k8stype.NodeName(pod.Spec.NodeName))
	}
}

func createNodeLabels(client clientset.Interface, namespace string, nodeSelectorList []*NodeSelector) (node1Name string, node1KeyValueLabel map[string]string, node2Name string, node2KeyValueLabel map[string]string) {
	nodes := framework.GetReadySchedulableNodesOrDie(client)
	if len(nodes.Items) < 2 {
		framework.Skipf("Requires at least %d nodes (not %d)", 2, len(nodes.Items))
	}
	for _, node := range nodes.Items {
		labelVal := "vsphere_e2e_" + string(uuid.NewUUID())
		nodeLabelMap := make(map[string]string)
		nodeLabelMap[NodeLabelKey] = labelVal
		nodeSelector := &NodeSelector{
			nodeName:  node.Name,
			nodeLabel: nodeLabelMap,
		}
		nodeSelectorList = append(nodeSelectorList, nodeSelector)
		framework.AddOrUpdateLabelOnNode(client, node.Name, NodeLabelKey, labelVal)
	}
}
