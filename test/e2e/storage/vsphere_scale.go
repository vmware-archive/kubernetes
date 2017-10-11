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
	k8stypes "k8s.io/apimachinery/pkg/types"
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

type NodeSelector struct {
	labelKey   string
	labelValue string
}

var _ = SIGDescribe("vcp at scale [Feature:vsphere] ", func() {
	f := framework.NewDefaultFramework("vcp-at-scale")

	var (
		client            clientset.Interface
		namespace         string
		areNodesLabeled   bool
		nodeSelectorList  []*NodeSelector
		volumeCount       int
		numberOfInstances int
		volumesPerPod     int
		nodeVolumeMapChan chan map[string][]string
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		nodeLabelMap = make(map[string]string)
		nodeVolumeMapChan = make(chan map[string][]string)
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
			areNodesLabeled = true
		}
	})

	It("vsphere scale tests", func() {
		pvcClaimList := make([]string)
		nodeVolumeMap := make(map[k8stypes.NodeName][]string)
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
			go VolumeCreateAndAttach(client, namespace, scArrays, volumeCountPerInstance, volumesPerPod, nodeSelectorList, nodeVolumeMapChan)
		}

		// Get the list of all volumes attached to each node from the go routines by reading the data from the channel
		for instanceCount := 0; instanceCount < 3; instanceCount++ {
			for node, volumeList := range <-nodeVolumeMapChan {
				nodeVolumeMap[k8stypes.NodeName(node)] = append(nodeVolumeMap[k8stypes.NodeName(node)], volumeList...)
			}
		}
		framework.Logf("balu - nodeVolumeMap: %+v", nodeVolumeMap)
		podList, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		framework.Logf("balu - podList: %+v", podList)
		for _, pod := range podList {
			pvcClaimList = append(pvcClaimList, getClaimsForPod(pod)...)
			By("Deleting pod")
			framework.DeletePodWithWait(f, client, pod)
		}
		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for volumes to be detached from the node")
		err = waitForVSphereDisksToDetach(vsp, nodeVolumeMap)
		Expect(err).NotTo(HaveOccurred())

		for _, pvcClaim := range pvcClaimList {
			framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)
		}
	})
})

// Get PVC claims for the pod
func getClaimsForPod(pod *v1.Pod) []string {
	pvcClaimList := make([]string)
	for _, volumespec := range pod.Spec.Volumes {
		if volumespec.PersistentVolumeClaim != nil {
			pvcClaimList = append(pvcClaimList, volumespec.PersistentVolumeClaim.ClaimName)
		}
	}
	return pvcClaimList
}

// VolumeCreateAndAttach peforms create and attach operations of vSphere persistent volumes at scale
func VolumeCreateAndAttach(client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumeCountPerInstance int, volumesPerPod int, nodeSelectorList []*NodeSelector, nodeVolumeMapChan chan map[string][]string) {
	nodeVolumeMap := make(map[string][]string)
	for index := 0; index < volumeCountPerInstance; index = index + volumesPerPod {
		pvclaims := make([]*v1.PersistentVolumeClaim, volumesPerPod)
		for i = 0; i < volumesPerPod; i++ {
			By(fmt.Sprintf("Creating PVC%q using the Storage Class", index+1))
			pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc[index%len(sc)]))
			Expect(err).NotTo(HaveOccurred())
			pvclaims = append(pvclaims, pvclaim)
		}

		By("Waiting for claim to be in bound phase")
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims)
		Expect(err).NotTo(HaveOccurred())

		By("Creating pod to attach PV to the node")
		nodeSelector := nodeSelectorList[index%len(nodeSelectorList)]
		// Create pod to attach Volume to Node
		pod, err := framework.CreatePod(client, namespace, map[string]string{nodeSelector.labelKey: nodeSelector.labelValue}, pvclaims, false, "")
		Expect(err).NotTo(HaveOccurred())

		for _, pv := range persistentvolumes {
			nodeVolumeMap[pod.Spec.NodeName] = append(nodeVolumeMap[pod.Spec.NodeName], pv.Spec.VsphereVolume.VolumePath)
		}
		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())
		By("Verify the volume is accessible and available in the pod")
		verifyVSphereVolumesAccessible(pod, persistentvolumes, vsp)
	}
	nodeVolumeMapChan <- nodeVolumeMap
}

func createNodeLabels(client clientset.Interface, namespace string, nodeSelectorList []*NodeSelector) {
	nodes := framework.GetReadySchedulableNodesOrDie(client)
	for _, node := range nodes.Items {
		labelVal := "vsphere_e2e_" + string(uuid.NewUUID())
		nodeSelector := &NodeSelector{
			labelKey:   NodeLabelKey,
			labelValue: labelVal,
		}
		nodeSelectorList = append(nodeSelectorList, nodeSelector)
		framework.Logf("balu - nodelabelkey: %q, nodelabelvalue: %q and nodeName: %q, nodeSelectorList: %+v", NodeLabelKey, labelVal, node.Name, nodeSelectorList)
		framework.AddOrUpdateLabelOnNode(client, node.Name, NodeLabelKey, labelVal)
	}
}
