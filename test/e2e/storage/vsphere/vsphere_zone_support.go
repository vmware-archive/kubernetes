/*
Copyright 2019 The Kubernetes Authors.

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

package vsphere

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

const (
	VsanDatastore1  = "vsanDatastore"
	VsanDatastore2  = "vsanDatastore (1)"
	CompatPolicy    = "compatpolicy"
	NonCompatPolicy = "noncompatpolicy"
	ZoneA           = "zone-a"
	ZoneB           = "zone-b"
	ZoneC           = "zone-c"
	ZoneD           = "zone-d"
)

/*
   Test to verify multi-zone support for dynamic volume provisioning in kubernetes.
   The test environment is illustrated below:

   datacenter
   	--->cluster-vsan-1 (zone-a)          	 --------------------
   		--->host-1 : master             |                    |
   		--->host-2 : node1      	|   vsanDatastore    |
   		--->host-3 : node2      	|____________________|


	--->cluster-vsan-2 (zone-b) 	  	 --------------------
   		--->host-4 : node3      	|                    |
   		--->host-5 : node4      	|  vsanDatastore (1) |
   		--->host-6       		|____________________|


	--->cluster-3 (zone-c)
		--->host-7 : node5       	-----------------
						| localDatastore |
						|                |
						-----------------

        --->host-8 (zone-c) : node6          	 --------------------
        					| localDatastore (1) |
        					|                    |
        					 --------------------
	Testbed description :
	1. cluster-vsan-1 is tagged with zone-a. So, vsanDatastore inherits zone-a since all the hosts under zone-a have vsanDatastore mounted on it.
	2. cluster-vsan-2 is tagged with zone-b. So, vsanDatastore (1) inherits zone-b since all the hosts under zone-b have vsanDatastore (1) mounted on it.
	3. cluster-3 is tagged with zone-c. cluster-3 only contains host-7.
	4. host-8 is not under any cluster and is tagged with zone-c.
	5. Since there are no shared datastores between host-7 under cluster-3 and host-8, no datastores in the environment inherit zone-c.
	6. The six worker nodes are ditributed among the hosts as shown in the above illustration
	7. Two storage policies are created on VC. One is a VSAN storage policy named as compatpolicy with hostFailuresToTolerate capability set to 1.
	   Second is a VSAN storage policy named as noncompatpolicy with hostFailuresToTolerate capability set to 4.

	Testsuite description :
	1. Tests to verify that zone labels are set correctly on a dynamically created PV.
	2. Tests to verify dynamic pv creation fails if availability zones are not specified or if there are no shared datastores under the specified zones.
	3. Tests to verify dynamic pv creation using availability zones works in combination with other storage class parameters such as storage policy,
	   datastore and VSAN capabilities.
	4. Tests to verify dynamic pv creation using availability zones fails in combination with other storage class parameters such as storage policy,
	   datastore and VSAN capabilities specifications when any of the former mentioned parameters are incompatible with the rest.
*/

var _ = utils.SIGDescribe("Zone Support", func() {
	f := framework.NewDefaultFramework("zone-support")
	var (
		client       clientset.Interface
		namespace    string
		scParameters map[string]string
		zones        []string
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		Bootstrap(f)
		client = f.ClientSet
		namespace = f.Namespace.Name
		scParameters = make(map[string]string)
		zones = make([]string, 0)
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		if !(len(nodeList.Items) > 0) {
			framework.Failf("Unable to find ready and schedulable Node")
		}
	})

	It("Verify dynamically created pv with allowed zones specified in storage class, shows the right zone information on its labels", func() {
		By(fmt.Sprintf("Creating storage class with the following zones : %s", ZoneA))
		zones = append(zones, ZoneA)
		storageclass, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("zone-sc", nil, zones))
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create storage class with err: %v", err))
		defer client.StorageV1().StorageClasses().Delete(storageclass.Name, nil)

		By("Creating PVC using the storage class")
		pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClass(namespace, "2Gi", storageclass))
		Expect(err).NotTo(HaveOccurred())
		defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)

		var pvclaims []*v1.PersistentVolumeClaim
		pvclaims = append(pvclaims, pvclaim)
		By("Waiting for claim to be in bound phase")
		persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims, framework.ClaimProvisionTimeout)
		Expect(err).NotTo(HaveOccurred())

		By("Verify zone information is present in the volume labels")
		for _, pv := range persistentvolumes {
			Expect(pv.ObjectMeta.Labels["failure-domain.beta.kubernetes.io/zone"]).To(Equal(ZoneA), "Incorrect or missing zone labels in pv.")
		}
	})

	It("Verify PVC creation with invalid zone specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with unknown zone : %s", ZoneD))
		zones = append(zones, ZoneD)
		err := verifyPVCCreationFails(client, namespace, nil, zones)
		Expect(err).To(HaveOccurred())
		errorMsg := "Failed to find a shared datastore matching zone [" + ZoneD + "]"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on allowed zones specified in storage class ", func() {
		By(fmt.Sprintf("Creating storage class with zones :%s", ZoneA))
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, nil, zones)
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on the allowed zones and datastore specified in storage class", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s and datastore :%s", ZoneA, VsanDatastore1))
		scParameters[Datastore] = VsanDatastore1
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, scParameters, zones)
	})

	It("Verify PVC creation with incompatible datastore and zone combination specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s and datastore :%s", ZoneC, VsanDatastore1))
		scParameters[Datastore] = VsanDatastore1
		zones = append(zones, ZoneC)
		err := verifyPVCCreationFails(client, namespace, scParameters, zones)
		errorMsg := "The specified datastore " + scParameters[Datastore] + " is not a shared datastore across node VMs or does not match the provided availability zones : [" + ZoneC + "]"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on the allowed zones and storage policy specified in storage class", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s and storage policy :%s", ZoneA, CompatPolicy))
		scParameters[SpbmStoragePolicy] = CompatPolicy
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, scParameters, zones)
	})

	It("Verify PVC creation with incompatible storagePolicy and zone combination specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s and storage policy :%s", ZoneA, NonCompatPolicy))
		scParameters[SpbmStoragePolicy] = NonCompatPolicy
		zones = append(zones, ZoneA)
		err := verifyPVCCreationFails(client, namespace, scParameters, zones)
		errorMsg := "No compatible datastores found that satisfy the storage policy requirements"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on the allowed zones, datastore and storage policy specified in storage class", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s datastore :%s and storagePolicy :%s", ZoneA, VsanDatastore1, CompatPolicy))
		scParameters[SpbmStoragePolicy] = CompatPolicy
		scParameters[Datastore] = VsanDatastore1
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, scParameters, zones)
	})

	It("Verify PVC creation with incompatible storage policy along with compatible zone and datastore combination specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s datastore :%s and storagePolicy :%s", ZoneA, VsanDatastore1, NonCompatPolicy))
		scParameters[SpbmStoragePolicy] = NonCompatPolicy
		scParameters[Datastore] = VsanDatastore1
		zones = append(zones, ZoneA)
		err := verifyPVCCreationFails(client, namespace, scParameters, zones)
		errorMsg := "User specified datastore is not compatible with the storagePolicy: \\\"" + NonCompatPolicy + "\\\"."
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation with incompatible zone along with compatible storagePolicy and datastore combination specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s datastore :%s and storagePolicy :%s", ZoneC, VsanDatastore2, CompatPolicy))
		scParameters[SpbmStoragePolicy] = CompatPolicy
		scParameters[Datastore] = VsanDatastore2
		zones = append(zones, ZoneC)
		err := verifyPVCCreationFails(client, namespace, scParameters, zones)
		errorMsg := "The specified datastore " + scParameters[Datastore] + " does not match the provided availability zones : [" + ZoneC + "]"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation fails if no zones are specified in the storage class (No shared datastores exist among all the nodes)", func() {
		By(fmt.Sprintf("Creating storage class with no zones"))
		err := verifyPVCCreationFails(client, namespace, nil, nil)
		errorMsg := "No shared datastores found in the Kubernetes cluster"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation fails if only datastore is specified in the storage class (No shared datastores exist among all the nodes)", func() {
		By(fmt.Sprintf("Creating storage class with datastore :%s", VsanDatastore1))
		scParameters[Datastore] = VsanDatastore1
		err := verifyPVCCreationFails(client, namespace, scParameters, nil)
		errorMsg := "No shared datastores found in the Kubernetes cluster"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation fails if only storage policy is specified in the storage class (No shared datastores exist among all the nodes)", func() {
		By(fmt.Sprintf("Creating storage class with storage policy :%s", CompatPolicy))
		scParameters[SpbmStoragePolicy] = CompatPolicy
		err := verifyPVCCreationFails(client, namespace, scParameters, nil)
		errorMsg := "No shared datastores found in the Kubernetes cluster"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation with compatible policy and datastore without any zones specified in the storage class fails (No shared datastores exist among all the nodes)", func() {
		By(fmt.Sprintf("Creating storage class with storage policy :%s and datastore :%s", CompatPolicy, VsanDatastore1))
		scParameters[SpbmStoragePolicy] = CompatPolicy
		scParameters[Datastore] = VsanDatastore1
		err := verifyPVCCreationFails(client, namespace, scParameters, nil)
		errorMsg := "No shared datastores found in the Kubernetes cluster"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify PVC creation fails if the availability zone specified in the storage class have no shared datastores under it.", func() {
		By(fmt.Sprintf("Creating storage class with zone :%s", ZoneC))
		zones = append(zones, ZoneC)
		err := verifyPVCCreationFails(client, namespace, nil, zones)
		errorMsg := "Failed to find a shared datastore matching zone [" + ZoneC + "]"
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on multiple zones specified in the storage class (One zone has shared datastores and other does not)", func() {
		By(fmt.Sprintf("Creating storage class with the following zones :%s and %s", ZoneA, ZoneC))
		zones = append(zones, ZoneC)
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, nil, zones)
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on the multiple zones specified in the storage class. (PV can be created on either of the zones specified)", func() {
		By(fmt.Sprintf("Creating storage class with the following zones :%s and %s", ZoneA, ZoneB))
		zones = append(zones, ZoneA)
		zones = append(zones, ZoneB)
		verifyPVCAndPodCreationSucceeds(client, namespace, nil, zones)
	})

	It("Verify PVC creation with an invalid VSAN capability along with a compatible zone combination specified in storage class fails", func() {
		By(fmt.Sprintf("Creating storage class with %s :%s and zone :%s", Policy_HostFailuresToTolerate, HostFailuresToTolerateCapabilityInvalidVal, ZoneA))
		scParameters[Policy_HostFailuresToTolerate] = HostFailuresToTolerateCapabilityInvalidVal
		zones = append(zones, ZoneA)
		err := verifyPVCCreationFails(client, namespace, scParameters, zones)
		errorMsg := "Invalid value for " + Policy_HostFailuresToTolerate + "."
		if !strings.Contains(err.Error(), errorMsg) {
			Expect(err).NotTo(HaveOccurred(), errorMsg)
		}
	})

	It("Verify a pod is created and attached to a dynamically created PV, based on a VSAN capability and compatible zone specified in storage class", func() {
		By(fmt.Sprintf("Creating storage class with %s :%s, %s :%s and zone :%s", Policy_ObjectSpaceReservation, ObjectSpaceReservationCapabilityVal, Policy_IopsLimit, IopsLimitCapabilityVal, HostFailuresToTolerateCapabilityInvalidVal, ZoneA))
		scParameters[Policy_ObjectSpaceReservation] = ObjectSpaceReservationCapabilityVal
		scParameters[Policy_IopsLimit] = IopsLimitCapabilityVal
		zones = append(zones, ZoneA)
		verifyPVCAndPodCreationSucceeds(client, namespace, scParameters, zones)
	})
})

func verifyPVCAndPodCreationSucceeds(client clientset.Interface, namespace string, scParameters map[string]string, zones []string) {
	storageclass, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("zone-sc", scParameters, zones))
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create storage class with err: %v", err))
	defer client.StorageV1().StorageClasses().Delete(storageclass.Name, nil)

	By("Creating PVC using the Storage Class")
	pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClass(namespace, "2Gi", storageclass))
	Expect(err).NotTo(HaveOccurred())
	defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)

	var pvclaims []*v1.PersistentVolumeClaim
	pvclaims = append(pvclaims, pvclaim)
	By("Waiting for claim to be in bound phase")
	persistentvolumes, err := framework.WaitForPVClaimBoundPhase(client, pvclaims, framework.ClaimProvisionTimeout)
	Expect(err).NotTo(HaveOccurred())

	By("Creating pod to attach PV to the node")
	pod, err := framework.CreatePod(client, namespace, nil, pvclaims, false, "")
	Expect(err).NotTo(HaveOccurred())

	By("Verify persistent volume was created on the right zone")
	verifyVolumeCreationOnRightZone(persistentvolumes, pod.Spec.NodeName, zones)

	By("Verify the volume is accessible and available in the pod")
	verifyVSphereVolumesAccessible(client, pod, persistentvolumes)

	By("Deleting pod")
	framework.DeletePodWithWait(f, client, pod)

	By("Waiting for volumes to be detached from the node")
	waitForVSphereDiskToDetach(persistentvolumes[0].Spec.VsphereVolume.VolumePath, pod.Spec.NodeName)
}

func verifyPVCCreationFails(client clientset.Interface, namespace string, scParameters map[string]string, zones []string) error {
	storageclass, err := client.StorageV1().StorageClasses().Create(getVSphereStorageClassSpec("zone-sc", scParameters, zones))
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create storage class with err: %v", err))
	defer client.StorageV1().StorageClasses().Delete(storageclass.Name, nil)

	By("Creating PVC using the Storage Class")
	pvclaim, err := framework.CreatePVC(client, namespace, getVSphereClaimSpecWithStorageClass(namespace, "2Gi", storageclass))
	Expect(err).NotTo(HaveOccurred())
	defer framework.DeletePersistentVolumeClaim(client, pvclaim.Name, namespace)

	var pvclaims []*v1.PersistentVolumeClaim
	pvclaims = append(pvclaims, pvclaim)

	By("Waiting for claim to be in bound phase")
	err = framework.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, client, pvclaim.Namespace, pvclaim.Name, framework.Poll, 2*time.Minute)
	Expect(err).To(HaveOccurred())

	eventList, err := client.CoreV1().Events(pvclaim.Namespace).List(metav1.ListOptions{})
	framework.Logf("Failure message : %+q", eventList.Items[0].Message)
	return fmt.Errorf("Failure message: %+q", eventList.Items[0].Message)
}
