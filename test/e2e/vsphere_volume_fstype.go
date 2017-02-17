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

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stype "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	vsphere "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
)

/*
	Test to verify diskformat specified in storage-class is being honored while volume creation.
	Valid and supported options are eagerzeroedthick, zeroedthick and thin

	Steps
	1. Create StorageClass with diskformat set to valid type
	2. Create PVC which uses the StorageClass created in step 1.
	3. Wait for PV to be provisioned.
	4. Wait for PVC's status to become Bound
	5. Create pod using PVC on specific node.
	6. Wait for Disk to be attached to the node.
	7. Get node VM's devices and find PV's Volume Disk.
	8. Get Backing Info of the Volume Disk and obtain EagerlyScrub and ThinProvisioned
	9. Based on the value of EagerlyScrub and ThinProvisioned, verify diskformat is correct.
	10. Delete pod and Wait for Volume Disk to be detached from the Node.
	11. Delete PVC, PV and Storage Class
*/

var _ = framework.KubeDescribe("Volume fstype [Volumes]", func() {
	f := framework.NewDefaultFramework("volume-fstype")
	var (
		client            clientset.Interface
		namespace         string
		nodeName          string
		isNodeLabeled     bool
		nodeKeyValueLabel map[string]string
		nodeLabelValue    string
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		if len(nodeList.Items) != 0 {
			nodeName = nodeList.Items[0].Name
		} else {
			framework.Failf("Unable to find ready and schedulable Node")
		}
		if !isNodeLabeled {
			nodeLabelValue := "vsphere_e2e_" + string(uuid.NewUUID())
			nodeKeyValueLabel = make(map[string]string)
			nodeKeyValueLabel["vsphere_e2e_label"] = nodeLabelValue
			framework.AddOrUpdateLabelOnNode(client, nodeName, "vsphere_e2e_label", nodeLabelValue)
			isNodeLabeled = true
		}
	})
	AddCleanupAction(func() {
		if len(nodeLabelValue) > 0 {
			framework.RemoveLabelOffNode(client, nodeName, "vsphere_e2e_label")
		}
	})

	It("verify disk format type - eagerzeroedthick is honored for dynamically provisioned pv using storageclass", func() {
		By("Invoking Test for diskformat: eagerzeroedthick")
		invokeTestForFstype(client, namespace, nodeName, nodeKeyValueLabel, "ext3", "ext3")
	})
	It("verify disk format type - zeroedthick is honored for dynamically provisioned pv using storageclass", func() {
		By("Invoking Test for diskformat: zeroedthick")
		invokeTestForFstype(client, namespace, nodeName, nodeKeyValueLabel, "", "ext4")
	})
})

func invokeTestForFstype(client clientset.Interface, namespace string, nodeName string, nodeKeyValueLabel map[string]string, diskFormat string, expectedContent string) {

	framework.Logf("Invoking Test for DiskFomat: %s", diskFormat)
	scParameters := make(map[string]string)
	scParameters["fstype"] = diskFormat

	By("Creating Storage Class With DiskFormat")
	storageClassSpec := getVSphereStorageClassSpec("thinsc", scParameters)
	storageclass, err := client.StorageV1beta1().StorageClasses().Create(storageClassSpec)
	Expect(err).NotTo(HaveOccurred())

	defer client.StorageV1beta1().StorageClasses().Delete(storageclass.Name, nil)

	By("Creating PVC using the Storage Class")
	pvclaimSpec := getVSphereClaimSpecWithStorageClassAnnotation(namespace, storageclass)
	pvclaim, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(pvclaimSpec)
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		client.CoreV1().PersistentVolumeClaims(namespace).Delete(pvclaimSpec.Name, nil)
	}()

	By("Waiting for claim to be in bound phase")
	err = framework.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, client, pvclaim.Namespace, pvclaim.Name, framework.Poll, framework.ClaimProvisionTimeout)
	Expect(err).NotTo(HaveOccurred())

	// Get new copy of the claim
	pvclaim, err = client.CoreV1().PersistentVolumeClaims(pvclaim.Namespace).Get(pvclaim.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Get the bound PV
	pv, err := client.CoreV1().PersistentVolumes().Get(pvclaim.Spec.VolumeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	/*
		PV is required to be attached to the Node. so that using govmomi API we can grab Disk's Backing Info
		to check EagerlyScrub and ThinProvisioned property
	*/
	By("Creating pod to attach PV to the node")
	// Create pod to attach Volume to Node
	podSpec := getVSpherePodSpecWithClaim(pvclaim.Name, nodeKeyValueLabel, "while true ; do sleep 2 ; done")
	pod, err := client.CoreV1().Pods(namespace).Create(podSpec)
	Expect(err).NotTo(HaveOccurred())

	vsp, err := vsphere.GetVSphere()
	Expect(err).NotTo(HaveOccurred())
	verifyVSphereDiskAttached(vsp, pv.Spec.VsphereVolume.VolumePath, k8stype.NodeName(nodeName))

	By("Waiting for pod to be running")
	Expect(framework.WaitForPodNameRunningInNamespace(client, pod.Name, namespace)).To(Succeed())
	_, err = framework.LookForStringInPodExec(namespace, pod.Name, []string{"df -T", pod.Spec.Containers[0].VolumeMounts[0].MountPath, "| awk 'FNR == 2 {print $2}'"}, expectedContent, time.Minute)
	By("Delete pod and wait for volume to be detached from node")
	deletePodAndWaitForVolumeToDetach(client, namespace, vsp, nodeName, pod, pv.Spec.VsphereVolume.VolumePath)

}
