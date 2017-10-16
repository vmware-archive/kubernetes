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
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	vsphere "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
)

/* This test calculates latency numbers for volume lifecycle operations
1. Create 4 type of storage classes
2. Read the total number of volumes to be created and volumes per pod
3. Create total PVCs (number of volumes)
4. Create Pods with attached volumes per pod
5. Verify access to the volumes
6. Delete pods and wait for volumes to detach
7. Delete the PVCs
*/
const (
	NodeLabelKey              = "vsphere_e2e_label"
	SCSIUnitsAvailablePerNode = 55
	NumOps                    = 4 //Number of volume operations whose latency is to be calculated
)

// NodeSelector holds
type NodeSelector struct {
	labelKey   string
	labelValue string
}

var _ = SIGDescribe("vcp-performance", func() {
	f := framework.NewDefaultFramework("vcp-performance")

	var (
		client           clientset.Interface
		namespace        string
		nodeSelectorList []*NodeSelector
		volumeCount      int
		volumesPerPod    int
		iterations       int
	)

	BeforeEach(func() {
		var err error
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name

		Expect(os.Getenv("VSPHERE_SPBM_GOLD_POLICY")).NotTo(BeEmpty(), "env VSPHERE_SPBM_GOLD_POLICY is not set")
		Expect(os.Getenv("VSPHERE_DATASTORE")).NotTo(BeEmpty(), "env VSPHERE_DATASTORE is not set")
		Expect(os.Getenv("VCP_PERF_VOLUME_PER_POD")).NotTo(BeEmpty(), "env VCP_PERF_VOLUME_PER_POD is not set")
		Expect(os.Getenv("VCP_PERF_ITERATIONS")).NotTo(BeEmpty(), "env VCP_PERF_ITERATIONS is not set")
		Expect(os.Getenv("VCP_PERF_VOLUME_COUNT")).NotTo(BeEmpty(), "env VCP_PERF_VOLUME_COUNT is not set")

		// Verify volume count specified by the user can be satisfied
		volumeCount, err = strconv.Atoi(os.Getenv("VCP_PERF_VOLUME_COUNT"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_PERF_VOLUME_COUNT")
		volumesPerPod, err = strconv.Atoi(os.Getenv("VCP_PERF_VOLUME_PER_POD"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_PERF_VOLUME_PER_POD")
		iterations, err = strconv.Atoi(os.Getenv("VCP_PERF_ITERATIONS"))
		Expect(err).NotTo(HaveOccurred(), "Error Parsing VCP_PERF_ITERATIONS")

		nodes := framework.GetReadySchedulableNodesOrDie(client)
		if len(nodes.Items) < 2 {
			framework.Skipf("Requires at least %d nodes (not %d)", 2, len(nodes.Items))
		}
		if volumeCount > SCSIUnitsAvailablePerNode*len(nodes.Items) {
			framework.Skipf("Cannot attach %d volumes to %d nodes. Maximum volumes that can be attached on %d nodes is %d", volumeCount, len(nodes.Items), len(nodes.Items), SCSIUnitsAvailablePerNode*len(nodes.Items))
		}
		nodeSelectorList = createNodeLabels(client, namespace, nodes)

	})

	It("vcp performance tests", func() {
		scList := getTestStorageClasses(client)

		var sumLatency [NumOps]int64
		for i := 0; i < iterations; i++ {
			latency := invokeVolumeLifeCyclePerformance(f, client, namespace, scList, volumesPerPod, volumeCount, nodeSelectorList)
			for i, val := range latency {
				sumLatency[i] += val
			}
		}

		iterations64 := int64(iterations)
		framework.Logf("Average latency for below operations")
		framework.Logf("Creating %v PVCs and waiting for bound phase: %v microseconds", volumeCount, sumLatency[0]/iterations64)
		framework.Logf("Creating %v Pod: %v microseconds", volumeCount/volumesPerPod, sumLatency[1]/iterations64)
		framework.Logf("Deleting %v Pod and waiting for disk to be detached: %v microseconds", volumeCount/volumesPerPod, sumLatency[2]/iterations64)
		framework.Logf("Deleting %v PVCs: %v microseconds", volumeCount, sumLatency[3]/iterations64)

		for _, sc := range scList {
			client.StorageV1().StorageClasses().Delete(sc.Name, nil)
		}
	})
})

func getTestStorageClasses(client clientset.Interface) []*storageV1.StorageClass {
	const (
		storageclass1 = "sc-default"
		storageclass2 = "sc-vsan"
		storageclass3 = "sc-spbm"
		storageclass4 = "sc-user-specified-ds"
	)
	scNames := []string{storageclass1, storageclass2, storageclass3, storageclass4}
	scArrays := make([]*storageV1.StorageClass, len(scNames))
	for index, scname := range scNames {
		// Create vSphere Storage Class
		By(fmt.Sprintf("Creating Storage Class : %v", scname))
		var sc *storageV1.StorageClass
		var err error
		switch scname {
		case storageclass1:
			sc, err = client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass1, nil))
		case storageclass2:
			var scVSanParameters map[string]string
			scVSanParameters = make(map[string]string)
			scVSanParameters[Policy_HostFailuresToTolerate] = "1"
			sc, err = client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass2, scVSanParameters))
		case storageclass3:
			var scSPBMPolicyParameters map[string]string
			scSPBMPolicyParameters = make(map[string]string)
			policyName := os.Getenv("VSPHERE_SPBM_GOLD_POLICY")
			Expect(policyName).NotTo(BeEmpty())
			scSPBMPolicyParameters[SpbmStoragePolicy] = policyName
			sc, err = client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec(storageclass3, scSPBMPolicyParameters))
		case storageclass4:
			var scWithDSParameters map[string]string
			scWithDSParameters = make(map[string]string)
			datastoreName := os.Getenv("VSPHERE_DATASTORE")
			Expect(datastoreName).NotTo(BeEmpty())
			scWithDSParameters[Datastore] = datastoreName
			scWithDatastoreSpec := getVSphereStorageClassSpec(storageclass4, scWithDSParameters)
			sc, err = client.StorageV1().StorageClasses().Create(scWithDatastoreSpec)
		}
		Expect(sc).NotTo(BeNil())
		Expect(err).NotTo(HaveOccurred())
		scArrays[index] = sc
	}
	return scArrays
}

// invokeVolumeLifeCyclePerformance peforms full volume life cycle management and records latency for each operation
func invokeVolumeLifeCyclePerformance(f *framework.Framework, client clientset.Interface, namespace string, sc []*storageV1.StorageClass, volumesPerPod int, volumeCount int, nodeSelectorList []*NodeSelector) []int64 {
	const timeFraction = 1000 // Show results in microseconds
	var (
		latency       []int64
		totalpvclaims [][]*v1.PersistentVolumeClaim
		totalpvs      [][]*v1.PersistentVolume
		totalpods     []*v1.Pod
	)
	nodeVolumeMap := make(map[types.NodeName][]string)

	numPods := volumeCount / volumesPerPod

	By(fmt.Sprintf("Creating %v PVCs", volumeCount))
	start := time.Now()
	for i := 0; i < numPods; i++ {
		var pvclaims []*v1.PersistentVolumeClaim
		for j := 0; j < volumesPerPod; j++ {
			currsc := sc[((i*numPods)+j)%len(sc)]
			pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClassAnnotation(namespace, currsc))
			Expect(err).NotTo(HaveOccurred())
			pvclaims = append(pvclaims, pvclaim)
		}
		totalpvclaims = append(totalpvclaims, pvclaims)
	}
	for _, pvclaims := range totalpvclaims {
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims, framework.ClaimProvisionTimeout)
		Expect(err).NotTo(HaveOccurred())
		totalpvs = append(totalpvs, persistentvolumes)
	}
	elapsed := time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	By("Creating pod to attach PVs to the node")
	start = time.Now()
	for i, pvclaims := range totalpvclaims {
		nodeSelector := nodeSelectorList[i%len(nodeSelectorList)]
		pod, err := framework.CreatePod(client, namespace, map[string]string{nodeSelector.labelKey: nodeSelector.labelValue}, pvclaims, false, "")
		Expect(err).NotTo(HaveOccurred())
		totalpods = append(totalpods, pod)
	}
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	// Verify access to the volumes
	vsp, err := vsphere.GetVSphere()
	Expect(err).NotTo(HaveOccurred())

	for i, pod := range totalpods {
		verifyVSphereVolumesAccessible(pod, totalpvs[i], vsp)
	}

	By("Deleting pods")
	start = time.Now()
	for _, pod := range totalpods {
		err = framework.DeletePodWithWait(f, client, pod)
		Expect(err).NotTo(HaveOccurred())
	}
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	for i, pod := range totalpods {
		for _, pv := range totalpvs[i] {
			nodeName := types.NodeName(pod.Spec.NodeName)
			nodeVolumeMap[nodeName] = append(nodeVolumeMap[nodeName], pv.Spec.VsphereVolume.VolumePath)
		}
	}

	err = waitForVSphereDisksToDetach(vsp, nodeVolumeMap)
	Expect(err).NotTo(HaveOccurred())

	By("Deleting the PVCs")
	start = time.Now()
	for _, pvclaims := range totalpvclaims {
		for _, pvc := range pvclaims {
			err = framework.DeletePersistentVolumeClaim(client, pvc.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		}
	}
	elapsed = time.Since(start)
	latency = append(latency, elapsed.Nanoseconds()/timeFraction)

	return latency
}

func createNodeLabels(client clientset.Interface, namespace string, nodes *v1.NodeList) []*NodeSelector {
	var nodeSelectorList []*NodeSelector
	for i, node := range nodes.Items {
		labelVal := "vsphere_e2e_" + strconv.Itoa(i)
		nodeSelector := &NodeSelector{
			labelKey:   NodeLabelKey,
			labelValue: labelVal,
		}
		nodeSelectorList = append(nodeSelectorList, nodeSelector)
		framework.AddOrUpdateLabelOnNode(client, node.Name, NodeLabelKey, labelVal)
	}
	return nodeSelectorList
}
