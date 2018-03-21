/*
Copyright 2018 The Kubernetes Authors.

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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

/*
	Test to verify that a volume remains attached through vpxd restart.

	For the number of schedulable nodes:
	1. Create a Volume with default options.
	2. Create a Pod with the created Volume.
	3. Verify that the Volume is attached.
	4. Create a file with random contents under the Volume's mount point on the Pod.
	5. Stop the vpxd service on the vCenter host.
	6. Verify that the file is accessible on the Pod and that it's contents match.
	7. Start the vpxd service on the vCenter host.
	8. Verify that the Volume remains attached, the file is accessible on the Pod, and that it's contents match.
	9. Delete the Pod and wait for the Volume to be detached.
	10. Delete the Volume.
*/
var _ = utils.SIGDescribe("Verify Volume Attach Through vpxd Restart [Feature:vsphere][Serial][Disruptive]", func() {
	f := framework.NewDefaultFramework("restart-vpxd")

	const (
		labelKey        = "vsphere_e2e_label"
		vpxdServiceName = "vmware-vpxd"
	)

	var (
		client                clientset.Interface
		namespace             string
		volumePaths           []string
		filePaths             []string
		fileContents          []string
		pods                  []*v1.Pod
		numNodes              int
		nodeKeyValueLabelList []map[string]string
		nodeNameList          []string
		nodeInfo              *NodeInfo
		vcHost                string
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		Bootstrap(f)
		client = f.ClientSet
		namespace = f.Namespace.Name
		framework.ExpectNoError(framework.WaitForAllNodesSchedulable(client, framework.TestContext.NodeSchedulableTimeout))

		nodes := framework.GetReadySchedulableNodesOrDie(client)
		numNodes = len(nodes.Items)
		if numNodes < 1 {
			framework.Skipf("Requires at least %d nodes (not %d)", 1, len(nodes.Items))
		}
		nodeInfo = TestContext.NodeMapper.GetNodeInfo(nodes.Items[0].Name)
		for i := 0; i < numNodes; i++ {
			nodeName := nodes.Items[i].Name
			nodeNameList = append(nodeNameList, nodeName)
			nodeLabelValue := "vsphere_e2e_" + string(uuid.NewUUID())
			nodeKeyValueLabel := make(map[string]string)
			nodeKeyValueLabel[labelKey] = nodeLabelValue
			nodeKeyValueLabelList = append(nodeKeyValueLabelList, nodeKeyValueLabel)
			framework.AddOrUpdateLabelOnNode(client, nodeName, labelKey, nodeLabelValue)
		}

		for host := range TestContext.VSphereInstances {
			vcHost = host
			break
		}
	})

	It("verify volume remains attached through vpxd restart", func() {
		for i := 0; i < numNodes; i++ {
			By(fmt.Sprintf("Creating test vsphere volume %d", i))
			volumePath, err := nodeInfo.VSphere.CreateVolume(&VolumeOptions{}, nodeInfo.DataCenterRef)
			Expect(err).NotTo(HaveOccurred())
			volumePaths = append(volumePaths, volumePath)

			By(fmt.Sprintf("Creating pod %d on node %v", i, nodeNameList[i]))
			podspec := getVSpherePodSpecWithVolumePaths([]string{volumePath}, nodeKeyValueLabelList[i], nil)
			pod, err := client.CoreV1().Pods(namespace).Create(podspec)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Waiting for pod %d to be ready", i))
			Expect(framework.WaitForPodNameRunningInNamespace(client, pod.Name, namespace)).To(Succeed())

			pod, err = client.CoreV1().Pods(namespace).Get(pod.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			pods = append(pods, pod)

			nodeName := pod.Spec.NodeName
			By(fmt.Sprintf("Verifying that volume %v is attached to node %v", volumePath, nodeName))
			expectVolumeToBeAttached(nodeName, volumePath)

			By(fmt.Sprintf("Creating a file with random content on the volume mounted on pod %d", i))
			filePath := fmt.Sprintf("/mnt/volume1/%v_vpxd_restart_test_%v.txt", namespace, strconv.FormatInt(time.Now().UnixNano(), 10))
			randomContent := fmt.Sprintf("Random Content -- %v", strconv.FormatInt(time.Now().UnixNano(), 10))
			err = writeContentToPodFile(namespace, pod.Name, filePath, randomContent)
			Expect(err).NotTo(HaveOccurred())
			filePaths = append(filePaths, filePath)
			fileContents = append(fileContents, randomContent)
		}

		By("Stopping vpxd on the vCenter host")
		vcAddress := vcHost + ":22"
		err := invokeVCenterServiceControl("stop", vpxdServiceName, vcAddress)
		Expect(err).NotTo(HaveOccurred(), "Unable to stop vpxd on the vCenter host")

		expectFilesToBeAccessible(namespace, pods, filePaths)
		expectFileContentsToMatch(namespace, pods, filePaths, fileContents)

		By("Starting vpxd on the vCenter host")
		err = invokeVCenterServiceControl("start", vpxdServiceName, vcAddress)
		Expect(err).NotTo(HaveOccurred(), "Unable to start vpxd on the vCenter host")

		expectVolumesToBeAttached(pods, volumePaths)
		expectFilesToBeAccessible(namespace, pods, filePaths)
		expectFileContentsToMatch(namespace, pods, filePaths, fileContents)

		for i, pod := range pods {
			volumePath := volumePaths[i]
			nodeName := pod.Spec.NodeName

			By(fmt.Sprintf("Deleting pod on node %s", nodeName))
			err = framework.DeletePodWithWait(f, client, pod)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Waiting for volume %s to be detached from node %s", volumePath, nodeName))
			err = waitForVSphereDiskToDetach(volumePath, nodeName)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Deleting volume %s", volumePath))
			err = nodeInfo.VSphere.DeleteVolume(volumePath, nodeInfo.DataCenterRef)
			Expect(err).NotTo(HaveOccurred())
		}
	})
})
