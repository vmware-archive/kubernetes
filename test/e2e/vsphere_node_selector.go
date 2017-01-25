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
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/apimachinery/pkg/types"
	vsphere "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/util/uuid"
)

var _ = framework.KubeDescribe("basic-vsphere-volume-plugin-tests", func() {
	f := framework.NewDefaultFramework("basic-vsphere-volume-plugin-tests")
	var (
		c                  clientset.Interface
		ns                 string
		volumePath         string
		node1Name          string
		node1LabelValue    string
		node1KeyValueLabel map[string]string

		node2Name          string
		node2LabelValue    string
		node2KeyValueLabel map[string]string

		isNodeLabeled bool
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		c = f.ClientSet
		ns = f.Namespace.Name
		framework.ExpectNoError(framework.WaitForAllNodesSchedulable(c, framework.TestContext.NodeSchedulableTimeout))

		if (!isNodeLabeled) {
			nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
			if (len(nodeList.Items) != 0) {
				node1Name = nodeList.Items[0].Name
				node2Name = nodeList.Items[1].Name
			} else {
				framework.Failf("Unable to find ready and schedulable Node")
			}
			node1LabelValue = "vsphere_e2e_" + string(uuid.NewUUID())
			node1KeyValueLabel = make(map[string]string)
			node1KeyValueLabel["vsphere_e2e_label"] = node1LabelValue
			framework.AddOrUpdateLabelOnNode(c, node1Name, "vsphere_e2e_label", node1LabelValue)

			node2LabelValue = "vsphere_e2e_" + string(uuid.NewUUID())
			node2KeyValueLabel = make(map[string]string)
			node2KeyValueLabel["vsphere_e2e_label"] = node2LabelValue
			framework.AddOrUpdateLabelOnNode(c, node2Name, "vsphere_e2e_label", node2LabelValue)

		}

	})

	AddCleanupAction(func() {
		By("Running clean up actions")
		if len(node1LabelValue) > 0 {
			framework.RemoveLabelOffNode(c, node1Name, "vsphere_e2e_label")
		}
		if len(node2LabelValue) > 0 {
			framework.RemoveLabelOffNode(c, node2Name, "vsphere_e2e_label")
		}
	})

	framework.KubeDescribe("Test Back-to-back pod creation/deletion with the same volume source on the same worker node", func() {
		var volumeoptions vsphere.VolumeOptions
		It("should provision pod on the node with matching label", func() {
			By("creating vmdk")
			vsp, err := vsphere.GetVSphere()
			Expect(err).NotTo(HaveOccurred())

			volumeoptions.CapacityKB = 2097152
			volumeoptions.Name = "e2e-disk" + time.Now().Format("20060102150405")
			volumeoptions.DiskFormat = "thin"

			volumePath, err = vsp.CreateVolume(&volumeoptions)
			Expect(err).NotTo(HaveOccurred())

			By("Creating pod on the node: " + node1Name)
			pod := getPodSpec(volumePath, node1KeyValueLabel)

			pod, err = c.CoreV1().Pods(ns).Create(pod)
			Expect(err).NotTo(HaveOccurred())
			By("Waiting for pod to be ready")
			Expect(f.WaitForPodRunning(pod.Name)).To(Succeed())

			By("Verify volume is attached to the node: " + node1Name)
			verifyVsphereDiskAttached(volumePath, types.NodeName(node1Name))

			By("Deleting pod")
			err = c.CoreV1().Pods(ns).Delete(pod.Name, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for volume to be detached from the node")
			waitForVSphereDiskToDetach(volumePath, types.NodeName(node1Name))

			By("Creating pod on the same node: " + node1Name)
			pod = getPodSpec(volumePath, node1KeyValueLabel)
			pod, err = c.CoreV1().Pods(ns).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for pod to be running")
			Expect(f.WaitForPodRunning(pod.Name)).To(Succeed())

			By("Verify volume is attached to the node: " + node1Name)
			verifyVsphereDiskAttached(volumePath, types.NodeName(node1Name))

			By("Deleting pod")
			err = c.CoreV1().Pods(ns).Delete(pod.Name, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for volume to be detached from the node: " + node1Name)
			waitForVSphereDiskToDetach(volumePath, types.NodeName(node1Name))

			By("Deleting vmdk")
			if len(volumePath) > 0 {
				vsp.DeleteVolume(volumePath)
			}
		})
	})
	framework.KubeDescribe("Test Back-to-back pod creation/deletion with the same volume source attach/detach to different worker nodes", func() {
		var volumeoptions vsphere.VolumeOptions
		It("should provision pod on the node with matching label", func() {
			By("creating vmdk")
			vsp, err := vsphere.GetVSphere()
			Expect(err).NotTo(HaveOccurred())

			volumeoptions.CapacityKB = 2097152
			volumeoptions.Name = "e2e-disk" + time.Now().Format("20060102150405")
			volumeoptions.DiskFormat = "thin"

			volumePath, err = vsp.CreateVolume(&volumeoptions)
			Expect(err).NotTo(HaveOccurred())

			By("Creating pod on the node: " + node1Name)
			pod := getPodSpec(volumePath, node1KeyValueLabel)

			pod, err = c.CoreV1().Pods(ns).Create(pod)
			Expect(err).NotTo(HaveOccurred())
			By("Waiting for pod to be ready")
			Expect(f.WaitForPodRunning(pod.Name)).To(Succeed())

			By("Verify volume is attached to the node: " + node1Name)
			verifyVsphereDiskAttached(volumePath, types.NodeName(node1Name))

			By("Deleting pod")
			err = c.CoreV1().Pods(ns).Delete(pod.Name, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for volume to be detached from the node")
			waitForVSphereDiskToDetach(volumePath, types.NodeName(node1Name))

			By("Creating pod on the another node: " + node2Name)
			pod = getPodSpec(volumePath, node2KeyValueLabel)
			pod, err = c.CoreV1().Pods(ns).Create(pod)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for pod to be running")
			Expect(f.WaitForPodRunning(pod.Name)).To(Succeed())

			By("Verify volume is attached to the node: " + node2Name)
			verifyVsphereDiskAttached(volumePath, types.NodeName(node2Name))

			By("Deleting pod")
			err = c.CoreV1().Pods(ns).Delete(pod.Name, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for volume to be detached from the node: " + node2Name)
			waitForVSphereDiskToDetach(volumePath, types.NodeName(node2Name))

			By("Deleting vmdk")
			if len(volumePath) > 0 {
				vsp.DeleteVolume(volumePath)
			}
		})
	})
})

func getPodSpec(volumePath string, keyValuelabel map[string]string) (*v1.Pod) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vsphere-e2e-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "vsphere-e2e-container-" + string(uuid.NewUUID()),
					Image:   "gcr.io/google-containers/test-webserver",
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "vsphere-volume",
							MountPath: "/mnt/vsphere-volume",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "vsphere-volume",
					VolumeSource: v1.VolumeSource{
						VsphereVolume: &v1.VsphereVirtualDiskVolumeSource{
							VolumePath:   volumePath,
							FSType:       "ext4",
						},
					},
				},
			},
		},
	}

	if (keyValuelabel != nil) {
		pod.Spec.NodeSelector = keyValuelabel
	}
	return pod
}