package storage

import (
	"fmt"
	"os"
	"strconv"

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
	Perform vsphere volume life cycle management at scale based on user configurable value for number of volumes.
	The following actions will be performed as part of this test.

	1. Create Storage Classes of 4 Categories (Default, SC with Non Default Datastore, SC with SPBM Policy, SC with VSAN Storage Capalibilies.)
	2. Read VCP_SCALE_VOLUME_COUNT from System Environment.
	3. Launch VCP_SCALE_INSTANCES goroutine for creating VCP_SCALE_VOLUME_COUNT volumes. Each goroutine is responsible for create/attach of VCP_SCALE_VOLUME_COUNT/VCP_SCALE_INSTANCES volumes.
	4. Read VCP_SCALE_VOLUMES_PER_POD from System Environment. Each pod will be have VCP_SCALE_VOLUMES_PER_POD attached to it.
	5. Once all the go routines are completed, we delete all the pods and volumes.
*/
const (
	NodeLabelKey              = "vsphere_e2e_label"
	SCSIUnitsAvailablePerNode = 55
)

// NodeSelector stores info about node and the node selector
type NodeSelector struct {
	nodeName  string
	nodeLabel map[string]string
}

type Channels struct {
	podListChan chan []*v1.Pod
	nodeVolumeMapChan chan map[string][]string
	pvClaimNameChan chan []string
}

var _ = SIGDescribe("vcp-at-scale", func() {
	f := framework.NewDefaultFramework("vcp-at-scale")

	var (
		client            clientset.Interface
		namespace         string
		areNodesLabeled   bool
		nodeSelectorList  []*NodeSelector
		volumeCount       int
		numberOfInstances int
		volumesPerPod     int
		channels Channels
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		channels = Channels {
			podListChan : make(chan []*v1.Pod),
			volPathNodeName : make(chan map[string]string),
			pvClaimName : make(chan []string),
		}
		Expect(os.Getenv("VCP_SCALE_VOLUME_COUNT")).NotTo(BeEmpty(), "ENV VCP_SCALE_VOLUME_COUNT is not set")
		Expect(os.Getenv("VSPHERE_SPBM_GOLD_POLICY")).NotTo(BeEmpty(), "ENV VSPHERE_SPBM_GOLD_POLICY is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "ENV VSPHERE_DATASTORE is not set")

		volumesPerPod, err := strconv.Atoi(os.Getenv("VCP_SCALE_VOLUME_PER_POD"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_SCALE_VOLUME_PER_POD")

		numberOfInstances, err = strconv.Atoi(os.Getenv("VCP_SCALE_INSTANCES"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_SCALE_INSTANCES")

		// Verify volume count specified by the user can be satisfied
		volumeCount, err = strconv.Atoi(os.Getenv("VCP_SCALE_VOLUME_COUNT"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_SCALE_VOLUME_COUNT")
		nodes := framework.GetReadySchedulableNodesOrDie(client)
		if len(nodes.Items) < 2 {
			framework.Skipf("Requires at least %d nodes (not %d)", 2, len(nodes.Items))
		}
		if volumeCount > SCSIUnitsAvailablePerNode*len(nodes.Items) {
			framework.Skipf("Cannot attach %d volumes to %d nodes. Maximum volumes that can be attached on %d nodes is %d", volumeCount, len(nodes.Items), len(nodes.Items), SCSIUnitsAvailablePerNode*len(nodes.Items))
		}
		if !areNodesLabeled {
			createNodeLabels(client, namespace, nodeSelectorList)
			isNodeLabeled = true
		}
	})

	It("vsphere scale tests", func() {
		podList []*v1.Pod
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

		volumeCountPerInstance := volumeCount / numberOfInstances
		for instanceCount := 0; instanceCount < numberOfInstances; instanceCount++ {
			if instanceCount == numberOfInstances-1 {
				volumeCountPerInstance = volumeCount
			}
			volumeCount = volumeCount - volumeCountPerInstance
			go PerformVolumeLifeCycleAtScale(client, namespace, scArrays, volumeCountPerInstance, volumesPerPod, nodeSelectorList, &channels)
		}
		for instanceCount := 0; instanceCount < numberOfInstances; instanceCount++ {
			podList = append(podList, <-podListChan...)
		}
	})
})

// PerformVolumeLifeCycleAtScale peforms full volume life cycle management at scale
func PerformVolumeLifeCycleAtScale(client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumeCountPerInstance int, volumesPerPod int, nodeSelectorList []*NodeSelector, channels *Channels) {
	podList := make([]*v1.Pod, (volumeCountPerInstance/volumesPerPod)+1)
	pvClaimNameList := make([]string, volumeCountPerInstance)
	nodeVolumeMap map[string][]string
	for index := 0; index < volumeCountPerInstance; index = index + volumesPerPod {
		pvclaims := make([]*v1.PersistentVolumeClaim, volumesPerPod)
		for i = 0; i < volumesPerPod; i++ {
			By(fmt.Sprintf("Creating PVC%q using the Storage Class", index+1))
			pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc[index%len(sc)]))
			Expect(err).NotTo(HaveOccurred())
			pvClaimNameList = append(pvClaimNameList, pvclaim.Name)
			pvclaims = append(pvclaims, pvclaim)
			// defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)
		}

		By("Waiting for claim to be in bound phase")
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims)
		Expect(err).NotTo(HaveOccurred())

		By("Creating pod to attach PV to the node")
		// Create pod to attach Volume to Node
		pod, err := framework.CreatePod(client, namespace, nodeSelectorList[index%len(nodeSelectorList)].nodeLabel, pvclaims, false, "")
		Expect(err).NotTo(HaveOccurred())

		for _, pv := range persistentvolumes {
			nodeVolumeMap[pod.Spec.NodeName] = append(nodeVolumeMap[pod.Spec.NodeName], pv.Spec.VsphereVolume.VolumePath)
		}
		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())
		By("Verify the volume is accessible and available in the pod")
		verifyVSphereVolumesAccessible(pod, persistentvolumes, vsp)
	}
	channels.podListChan <- podList
	channels.volPathNodeNameChan <- pvClaimNameList
	channels.pvClaimNameChan <- nodeVolumeMap
}

func createNodeLabels(client clientset.Interface, namespace string, nodeSelectorList []*NodeSelector) (node1Name string, node1KeyValueLabel map[string]string, node2Name string, node2KeyValueLabel map[string]string) {
	nodes := framework.GetReadySchedulableNodesOrDie(client)
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
