package storage

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8stype "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
	"os"
	"strconv"
	"sync"
)

/*
	Induce stress to create volumes in parallel with multiple threads based on user configurable values for number of threads and iterations per thread.
	The following actions will be performed as part of this test.

	1. Create Storage Classes of 4 Categories (Default, SC with Non Default Datastore, SC with SPBM Policy, SC with VSAN Storage Capalibilies.)
    2. READ VCP_STRESS_INSTANCES and VCP_STRESS_ITERATIONS from System Environment.
	3. Launch goroutine for volume lifecycle operations.
	4. Each instance of routine iterates for n times, where n is read from system env - VCP_STRESS_ITERATIONS
	5. Each iteration creates 1 PVC, 1 POD using the provisioned PV, Verify disk is attached to the node, Verify pod can access the volume, delete the pod and finally delete the PVC.
*/
var _ = SIGDescribe("vsphere cloud provider stress [Feature:vsphere]", func() {
	f := framework.NewDefaultFramework("vcp-stress")
	const (
		volumesPerNode = 55
		storageclass1  = "sc-default"
		storageclass2  = "sc-vsan"
		storageclass3  = "sc-spbm"
		storageclass4  = "sc-user-specified-ds"
	)
	var (
		client     clientset.Interface
		namespace  string
		instances  int
		iterations int
		err        error
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name

		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		Expect(nodeList.Items).NotTo(BeEmpty(), "Unable to find ready and schedulable Node")

		// if VCP_STRESS_INSTANCES = 12 and VCP_STRESS_ITERATIONS is 10. 12 threads will run in parallel for 10 times.
		// Resulting 120 Volumes and POD Creation. Volumes will be provisioned with each different types of Storage Class,
		// Each iteration creates PVC, verify PV is provisioned, then creates a pod, verify volume is attached to the node, and then delete the pod and delete pvc.

		Expect(os.Getenv("VCP_STRESS_INSTANCES")).NotTo(BeEmpty(), "ENV VCP_STRESS_INSTANCES is not set")
		instances, err = strconv.Atoi(os.Getenv("VCP_STRESS_INSTANCES"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP-STRESS-INSTANCES")
		Expect(instances <= volumesPerNode*len(nodeList.Items)).To(BeTrue(), fmt.Sprintf("Number of Instances should be less or equal: %v", volumesPerNode*len(nodeList.Items)))
		Expect(instances > 3).To(BeTrue(), "VCP_STRESS_INSTANCES should be greater than 3 to utilize all 4 types of storage classes")

		Expect(os.Getenv("VCP_STRESS_ITERATIONS")).NotTo(BeEmpty(), "ENV VCP_STRESS_ITERATIONS is not set")
		iterations, err = strconv.Atoi(os.Getenv("VCP_STRESS_ITERATIONS"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_STRESS_ITERATIONS")
		Expect(iterations > 0).To(BeTrue(), "VCP_STRESS_ITERATIONS should be greater than 0")

		Expect(os.Getenv("VSPHERE_SPBM_POLICY_NAME")).NotTo(BeEmpty(), "ENV VSPHERE_SPBM_POLICY_NAME is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "ENV VSPHERE_DATASTORE is not set")
	})

	It("vsphere stress tests", func() {
		scArrays := make([]*storageV1.StorageClass, 4)
		// Create default vSphere Storage Class
		By(fmt.Sprintf("Creating Storage Class : %v", storageclass1))
		scDefault, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass1, nil))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(storageclass1, nil)
		scArrays[0] = scDefault

		// Create Storage Class with vSan Parameters
		By(fmt.Sprintf("Creating Storage Class : %v", storageclass2))
		var scVSanParameters map[string]string
		scVSanParameters = make(map[string]string)
		scVSanParameters[Policy_HostFailuresToTolerate] = "1"
		scVSan, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass2, scVSanParameters))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(storageclass2, nil)
		scArrays[1] = scVSan

		// Create Storage Class with SPBM Policy
		By(fmt.Sprintf("Creating Storage Class : %v", storageclass3))
		var scSPBMPolicyParameters map[string]string
		scSPBMPolicyParameters = make(map[string]string)
		spbmpolicy := os.Getenv("VSPHERE_SPBM_POLICY_NAME")
		Expect(spbmpolicy).NotTo(BeEmpty())
		scSPBMPolicyParameters[SpbmStoragePolicy] = spbmpolicy
		scSPBMPolicy, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass3, scSPBMPolicyParameters))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(storageclass3, nil)
		scArrays[2] = scSPBMPolicy

		// Create Storage Class with User Specified Datastore.
		By(fmt.Sprintf("Creating Storage Class : %v", storageclass4))
		var scWithDSParameters map[string]string
		scWithDSParameters = make(map[string]string)
		datastore := os.Getenv("VSPHERE_DATASTORE")
		Expect(datastore).NotTo(BeEmpty())
		scWithDSParameters[Datastore] = datastore
		scWithDatastoreSpec := getVSphereStorageClassSpec(storageclass4, scWithDSParameters)
		scWithDatastore, err := client.StorageV1().StorageClasses().Create(scWithDatastoreSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(storageclass4, nil)
		scArrays[3] = scWithDatastore

		var wg sync.WaitGroup
		wg.Add(instances)
		for instanceCount := 0; instanceCount < instances; instanceCount++ {
			instanceId := fmt.Sprintf("Thread:%v", instanceCount+1)
			go PerformVolumeLifeCycleInParallel(f, client, namespace, instanceId, scArrays[instanceCount%len(scArrays)], iterations, &wg)
		}
		wg.Wait()
	})

})

func PerformVolumeLifeCycleInParallel(f *framework.Framework, client clientset.Interface, namespace string, instanceId string, sc *storageV1.StorageClass, iterations int, wg *sync.WaitGroup) {
	defer wg.Done()
	defer GinkgoRecover()
	vsp, err := vsphere.GetVSphere()
	Expect(err).NotTo(HaveOccurred())

	for iterationCount := 0; iterationCount < iterations; iterationCount++ {
		logPrefix := fmt.Sprintf("Instance: [%v], Iteration: [%v] :", instanceId, iterationCount+1)
		By(fmt.Sprintf("%v Creating PVC using the Storage Class: %v", logPrefix, sc.Name))
		pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc))
		Expect(err).NotTo(HaveOccurred())
		defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)

		var pvclaims []*v1.PersistentVolumeClaim
		pvclaims = append(pvclaims, pvclaim)
		By(fmt.Sprintf("%v Waiting for claim: %v to be in bound phase", logPrefix, pvclaim.Name))
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Creating Pod using the claim: %v", logPrefix, pvclaim.Name))
		// Create pod to attach Volume to Node
		pod, err := framework.CreatePod(client, namespace, pvclaims, false, "")
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Waiting for the Pod: %v to be in the running state", logPrefix, pod.Name))
		Expect(f.WaitForPodRunningSlow(pod.Name)).NotTo(HaveOccurred())

		// Get the copy of the Pod to know the assigned node name.
		pod, err = client.CoreV1().Pods(namespace).Get(pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Verifing the volume: %v is attached to the node VM: %v", logPrefix, persistentvolumes[0].Spec.VsphereVolume.VolumePath, pod.Spec.NodeName))
		isVolumeAttached, verifyDiskAttachedError := verifyVSphereDiskAttached(vsp, persistentvolumes[0].Spec.VsphereVolume.VolumePath, types.NodeName(pod.Spec.NodeName))
		Expect(isVolumeAttached).To(BeTrue())
		Expect(verifyDiskAttachedError).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Verifing the volume: %v is accessible in the pod: %v", logPrefix, persistentvolumes[0].Spec.VsphereVolume.VolumePath, pod.Name))
		By("Verify the volume is accessible and available in the pod")
		verifyVSphereVolumesAccessible(pod, persistentvolumes, vsp)

		By(fmt.Sprintf("%v Deleting pod: %v", logPrefix, pod.Name))
		err = framework.DeletePodWithWait(f, client, pod)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Waiting for volume: %v to be detached from the node: %v", logPrefix, persistentvolumes[0].Spec.VsphereVolume.VolumePath, pod.Spec.NodeName))
		err = waitForVSphereDiskToDetach(vsp, persistentvolumes[0].Spec.VsphereVolume.VolumePath, k8stype.NodeName(pod.Spec.NodeName))
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("%v Deleting the Claim: %v", logPrefix, pvclaim.Name))
		Expect(framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)).NotTo(HaveOccurred())
	}
}
