package storage

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"os"
	"strconv"
	"sync"
)

/*
	Induce stress to create volumes in parallel with multiple threads based on user configurable values for number of volumes and threads.
	The following actions will be performed as part of this test.

	1. Create Storage Classes of 4 Categories (Default, SC with Non Default Datastore, SC with SPBM Policy, SC with VSAN Storage Capalibilies.)
    2. READ VCP_STRESS_INSTANCES and VCP_STRESS_VOLUMES_PER_INSTANCE from System Environment.
	3. Launch goroutine for creating volumes n times. Here n is the value specified by the user in the system env VCP_STRESS_INSTANCES env variable.
	4. Each instance of routine creates m number of volumes using the storage classes created in step-1. Here m is value specified by the user in the system env VCP_STRESS_VOLUMES_PER_INSTANCE.
*/
var _ = SIGDescribe("vcp-stress [Feature:vsphere]", func() {
	f := framework.NewDefaultFramework("vcp-stress")
	var (
		client    clientset.Interface
		namespace string
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		Expect(os.Getenv("VCP_STRESS_INSTANCES")).NotTo(BeEmpty(), "ENV VCP_STRESS_INSTANCES is not set")
		Expect(os.Getenv("VCP_STRESS_VOLUMES_PER_INSTANCE")).NotTo(BeEmpty(), "ENV VCP_STRESS_VOLUMES_PER_INSTANCE is not set")
		Expect(os.Getenv("VSPHERE_SPBM_POLICY_NAME")).NotTo(BeEmpty(), "ENV VSPHERE_SPBM_POLICY_NAME is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "ENV VSPHERE_DATASTORE is not set")
	})

	It("vsphere stress tests", func() {

		// if VCP_STRESS_INSTANCES = 10 and VCP_STRESS_VOLUMES_PER_INSTANCE is 20, 200 Volumes Will Created.
		// Volumes will be provisioned with each different types of Storage Class, in this case 50 volumes per SC.

		scArrays := make([]*storageV1.StorageClass, 4)
		// Create default vSphere Storage Class
		By("Creating Storage Class : sc-default")
		scDefault, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("sc-default", nil))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scDefault.Name, nil)
		scArrays[0] = scDefault

		// Create Storage Class with vSan Parameters
		By("Creating Storage Class : sc-vsan")
		var scVSanParameters map[string]string
		scVSanParameters = make(map[string]string)
		scVSanParameters[Policy_HostFailuresToTolerate] = "1"
		scVSan, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("sc-vsan", scVSanParameters))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scVSan.Name, nil)
		scArrays[1] = scVSan

		// Create Storage Class with SPBM Policy
		By("Creating Storage Class : sc-spbm")
		var scSPBMPolicyParameters map[string]string
		scSPBMPolicyParameters = make(map[string]string)
		spbmpolicy := os.Getenv("VSPHERE_SPBM_POLICY_NAME")
		Expect(spbmpolicy).NotTo(BeEmpty())
		scSPBMPolicyParameters[SpbmStoragePolicy] = spbmpolicy
		scSPBMPolicy, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("sc-spbm", scSPBMPolicyParameters))
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scSPBMPolicy.Name, nil)
		scArrays[2] = scSPBMPolicy

		// Create Storage Class with User Specified Datastore.
		By("Creating Storage Class : sc-user-specified-ds")
		var scWithDSParameters map[string]string
		scWithDSParameters = make(map[string]string)
		datastore := os.Getenv("VSPHERE_DATASTORE")
		Expect(datastore).NotTo(BeEmpty())
		scWithDSParameters[Datastore] = datastore
		scWithDatastoreSpec := getVSphereStorageClassSpec("sc-user-specified-ds", scWithDSParameters)
		scWithDatastore, err := client.StorageV1().StorageClasses().Create(scWithDatastoreSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(scWithDatastore.Name, nil)
		scArrays[3] = scWithDatastore

		instances, err := strconv.Atoi(os.Getenv("VCP_STRESS_INSTANCES"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP-STRESS-INSTANCES")

		volumesPerInstance, err := strconv.Atoi(os.Getenv("VCP_STRESS_VOLUMES_PER_INSTANCE"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_STRESS_VOLUMES_PER_INSTANCE")

		var wg sync.WaitGroup
		wg.Add(instances)
		for instanceCount := 0; instanceCount < instances; instanceCount++ {
			go PerformVolumeLifeCycleInParallel(client, namespace, scArrays, volumesPerInstance, &wg)
		}
		wg.Wait()
	})

})

func PerformVolumeLifeCycleInParallel(client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumesPerInstance int, wg *sync.WaitGroup) {
	defer wg.Done()
	pvclaims := make([]*v1.PersistentVolumeClaim, volumesPerInstance)
	for index := 0; index < volumesPerInstance; index++ {
		By("Creating PVC using the Storage Class")
		pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc[index%len(sc)]))
		Expect(err).NotTo(HaveOccurred())
		pvclaims[index] = pvclaim
		defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)
	}
	By("Waiting for claims status in this thread to become Bound")
	for _, claim := range pvclaims {
		err := framework.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, client, claim.Namespace, claim.Name, framework.Poll, framework.ClaimProvisionTimeout)
		Expect(err).NotTo(HaveOccurred())
	}
}
