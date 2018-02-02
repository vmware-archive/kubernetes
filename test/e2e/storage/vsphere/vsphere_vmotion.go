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

package vsphere

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

const (
	MasterRole    = "master"
	NodeRole      = "node"
)

var _ = utils.SIGDescribe("VM vMotion [Feature:vsphere]", func() {
	f := framework.NewDefaultFramework("vm-vmotion")
	var (
		client            clientset.Interface
		namespace         string
		nodeName 		  string
		masterName		  string
		isNodeLabeled     bool
		nodeLabelValue    string
		nodeKeyValueLabel map[string]string
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		masters, nodes := framework.GetMasterAndWorkerNodesOrDie(f.ClientSet)
		if masters.Len() != 0 {
			masterName = masters.List()[0]
		} else {
			framework.Failf("Unable to find masters")
		}
		if len(nodes.Items) != 0 {
			nodeName = nodes.Items[0].Name
		} else {
			framework.Failf("Unable to find ready and schedulable Node")
		}
		if !isNodeLabeled {
			nodeLabelValue = "vsphere_e2e_" + string(uuid.NewUUID())
			nodeKeyValueLabel = make(map[string]string)
			nodeKeyValueLabel["vsphere_e2e_label"] = nodeLabelValue
			framework.AddOrUpdateLabelOnNode(client, nodeName, "vsphere_e2e_label", nodeLabelValue)
			isNodeLabeled = true
		}

	})
	framework.AddCleanupAction(func() {
		// Cleanup actions will be called even when the tests are skipped and leaves namespace unset.
		if len(namespace) > 0 && len(nodeLabelValue) > 0 {
			framework.RemoveLabelOffNode(client, nodeName, "vsphere_e2e_label")
		}
	})

	utils.SIGDescribe("vmvmotion:vsphere", func() {
		/*
			Test Steps:
			1. Create vmdk
			2. Create PV Spec with volume path set to VMDK file created in Step-1, and PersistentVolumeReclaimPolicy is set to Delete
			3. Create PVC with the storage request set to PV's storage capacity.
			4. Wait for PV and PVC to bound.
			5. Delete PVC.
			6. Verify volume is attached to the node and volume is accessible in the pod.
			7. Verify PV status should be failed.
			8. Delete the pod.
			9. Verify PV should be detached from the node and automatically deleted.
		*/
		It("verify pod can be created with vsphere volume attached when master vm is migrated to different vc", func() {
			var (
				vsp *vsphere.VSphere
			)
			BeforeEach(func() {
				var err error
				vsp, err = getVSphere(client)
				Expect(err).NotTo(HaveOccurred())
				migrateVM(vsp, masterName)
			})

			invokeVMotionTest(f, client, namespace, nodeKeyValueLabel, vsp)
		})


	})
})

func migrateVM(vsp *vsphere.VSphere, nodeName string) {
	// nodeInfo, err := vsp.NodeManager().GetNodeInfo(k8stypes.NodeName(nodeName))
	// migrate utility
}

func invokeVMotionTest(f *framework.Framework, c clientset.Interface, ns string, nodeKeyValueLabel map[string]string, vsp *vsphere.VSphere) {

	_, pv, pvc, err := provisionStaticVolume(vsp, c, ns, v1.PersistentVolumeReclaimDelete)
	Expect(err).NotTo(HaveOccurred())
	// Wait for PV and PVC to Bind
	framework.ExpectNoError(framework.WaitOnPVandPVC(c, ns, pv, pvc))

	By("Creating the Pod")
	pod, err := framework.CreatePod(c, ns, nodeKeyValueLabel, []*v1.PersistentVolumeClaim{pvc}, false, "")
	Expect(err).NotTo(HaveOccurred())


	By("Deleting the Pod")
	framework.ExpectNoError(framework.DeletePodWithWait(f, c, pod), "Failed to delete pod ", pod.Name)

	By("Verify volume is detached from the node after Pod is deleted")
	Expect(waitForVSphereDiskToDetach(c, vsp, pv.Spec.VsphereVolume.VolumePath, types.NodeName(pod.Spec.NodeName))).NotTo(HaveOccurred())

	By("Deleting the Claim")
	framework.ExpectNoError(framework.DeletePersistentVolumeClaim(c, pvc.Name, ns), "Failed to delete PVC ", pvc.Name)
	pvc = nil

	By("Verify PV should be deleted automatically")
	framework.ExpectNoError(framework.WaitForPersistentVolumeDeleted(c, pv.Name, 1*time.Second, 30*time.Second))
	pv = nil

}

// Test Setup for persistentvolumereclaim tests for vSphere Provider
func provisionStaticVolume(vsp *vsphere.VSphere, c clientset.Interface, ns string, persistentVolumeReclaimPolicy v1.PersistentVolumeReclaimPolicy) (volumePath string, pv *v1.PersistentVolume, pvc *v1.PersistentVolumeClaim, err error) {
	By("provision static volume")
	By("creating vmdk")
	volumePath, err = createVSphereVolume(vsp, nil)
	if err != nil {
		return
	}
	By("creating the pv")
	pv = getVSpherePersistentVolumeSpec(volumePath, persistentVolumeReclaimPolicy, nil)
	pv, err = c.CoreV1().PersistentVolumes().Create(pv)
	if err != nil {
		return
	}
	By("creating the pvc")
	pvc = getVSpherePersistentVolumeClaimSpec(ns, nil)
	pvc, err = c.CoreV1().PersistentVolumeClaims(ns).Create(pvc)
	return
}