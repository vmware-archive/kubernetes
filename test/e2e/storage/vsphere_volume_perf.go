/*
Copyright 2017 The Kubernetes Authors.

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

package storage

import (
	"fmt"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	k8stype "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	vsphere "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = SIGDescribe("vcp-performance", func() {
	f := framework.NewDefaultFramework("vcp-performance")

	var (
		client    clientset.Interface
		namespace string
		//iterations int
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name

		Expect(os.Getenv("VSPHERE_SPBM_GOLD_POLICY")).NotTo(BeEmpty(), "env VSPHERE_SPBM_GOLD_POLICY is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "env VSPHERE_DATASTORE is not set")
		Expect(os.Getenv("VCP_PERF_VOLUME_PER_POD")).NotTo(BeEmpty(), "env VCP_PERF_VOLUME_PER_POD is not set")
		Expect(os.Getenv("VCP_PERF_ITERATIONS")).NotTo(BeEmpty(), "env VCP_PERF_ITERATIONS is not set")

		nodes := framework.GetReadySchedulableNodesOrDie(client)
		if len(nodes.Items) < 2 {
			framework.Skipf("Requires at least %d nodes (not %d)", 2, len(nodes.Items))
		}
	})

	It("vcp performance tests", func() {
		scList := getTestStorageClasses(client)

		volumesPerPod, err := strconv.Atoi(os.Getenv("VCP_PERF_VOLUME_PER_POD"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_PERF_VOLUME_PER_POD")
		iterations, err := strconv.Atoi(os.Getenv("VCP_PERF_ITERATIONS"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_PERF_ITERATIONS")

		for i := 0; i < iterations; i++ {
			latency := invokeVolumeLifeCyclePerformance(f, client, namespace, scList, volumesPerPod)
			framework.Logf("Performance numbers for iteration %d", i)
			framework.Logf("Creating PVCs and waiting for bound phase: %v microseconds", latency[0])
			framework.Logf("Creating Pod and verifying attached status: %v microseconds", latency[1])
			framework.Logf("Deleting Pod and waiting for disk to be detached: %v microseconds", latency[2])
			framework.Logf("Deleting the PVCs: %v microseconds", latency[3])
		}
		for _, sc := range scList {
			client.StorageV1().StorageClasses().Delete(sc.Name, nil)
		}
	})
})

func getTestStorageClasses(client clientset.Interface) []*storageV1.StorageClass {
	// Volumes will be provisioned with each different types of Storage Class
	var scParamsList []*storageV1.StorageClass
	var scList []*storageV1.StorageClass

	// Create default vSphere Storage Class
	By("Creating Storage Class : sc-default")
	scDefaultSpec := getVSphereStorageClassSpec("sc-default", nil)
	scParamsList = append(scParamsList, scDefaultSpec)

	// Create Storage Class with vsan storage capabilities
	By("Creating Storage Class : sc-vsan")
	scvsanParameters := make(map[string]string)
	scvsanParameters[Policy_HostFailuresToTolerate] = "1"
	scvsanSpec := getVSphereStorageClassSpec("sc-vsan", scvsanParameters)
	scParamsList = append(scParamsList, scvsanSpec)

	// Create Storage Class with SPBM Policy
	By("Creating Storage Class : sc-spbm")
	scSpbmPolicyParameters := make(map[string]string)
	goldPolicy := os.Getenv("VSPHERE_SPBM_GOLD_POLICY")
	Expect(goldPolicy).NotTo(BeEmpty())
	scSpbmPolicyParameters[SpbmStoragePolicy] = goldPolicy
	scSpbmPolicySpec := getVSphereStorageClassSpec("sc-spbm", scSpbmPolicyParameters)
	scParamsList = append(scParamsList, scSpbmPolicySpec)

	// Create Storage Class with User Specified Datastore.
	By("Creating Storage Class : sc-user-specified-datastore")
	scWithDatastoreParameters := make(map[string]string)
	datastore := os.Getenv("VSPHERE_DATASTORE")
	Expect(goldPolicy).NotTo(BeEmpty())
	scWithDatastoreParameters[Datastore] = datastore
	scWithDatastoreSpec := getVSphereStorageClassSpec("sc-user-specified-ds", scWithDatastoreParameters)
	scParamsList = append(scParamsList, scWithDatastoreSpec)

	for _, params := range scParamsList {
		storageClass, err := client.StorageV1().StorageClasses().Create(params)
		Expect(err).NotTo(HaveOccurred())
		scList = append(scList, storageClass)
	}

	return scList
}

// invokeVolumeLifeCyclePerformance peforms full volume life cycle management and records latency for each operation
func invokeVolumeLifeCyclePerformance(f *framework.Framework, client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumesPerPod int) []int64 {
	const timeFraction = 1000 // Show results in microseconds
	var latency []int64

	By(fmt.Sprintf("Creating %v PVCs per pod ", volumesPerPod))
	var pvclaims []*v1.PersistentVolumeClaim
	start := time.Now()
	for i := 0; i < volumesPerPod; i++ {
		pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, sc[i%len(sc)]))
		Expect(err).NotTo(HaveOccurred())
		pvclaims = append(pvclaims, pvclaim)
	}

	persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims, framework.ClaimProvisionTimeout)
	Expect(err).NotTo(HaveOccurred())
	elapsed := time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	By("Creating pod to attach PVs to the node and verifying access")
	start = time.Now()
	pod, err := framework.CreatePod(client, namespace, pvclaims, false, "")
	Expect(err).NotTo(HaveOccurred())

	vsp, err := vsphere.GetVSphere()
	Expect(err).NotTo(HaveOccurred())

	verifyVSphereVolumesAccessible(pod, persistentvolumes, vsp)
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	By("Deleting pod and waiting for volumes to be detached from the node")
	start = time.Now()
	err = framework.DeletePodWithWait(f, client, pod)
	Expect(err).NotTo(HaveOccurred())

	for _, pv := range persistentvolumes {
		err = waitForVSphereDiskToDetach(vsp, pv.Spec.VsphereVolume.VolumePath, k8stype.NodeName(pod.Spec.NodeName))
		Expect(err).NotTo(HaveOccurred())
	}
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	By("Deleting the PVCs")
	start = time.Now()
	for _, pvc := range pvclaims {
		err = framework.DeletePersistentVolumeClaim(client, pvc.Name, namespace)
		Expect(err).NotTo(HaveOccurred())
	}
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	return latency
}
