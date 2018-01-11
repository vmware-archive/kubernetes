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
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/find"
	"golang.org/x/net/context"

	vimtypes "github.com/vmware/govmomi/vim25/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	"time"
	"github.com/vmware/govmomi/vim25/mo"
)

var _ = utils.SIGDescribe("Node Delete [Feature:vsphere] [Slow] [Disruptive]", func() {
	f := framework.NewDefaultFramework("node-unregister")
	var (
		client     clientset.Interface
		namespace  string
		vsp        *vsphere.VSphere
		workingDir string
		err        error
	)

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		client = f.ClientSet
		namespace = f.Namespace.Name
		framework.ExpectNoError(framework.WaitForAllNodesSchedulable(client, framework.TestContext.NodeSchedulableTimeout))
		vsp, err = getVSphere(client)
		Expect(err).NotTo(HaveOccurred())
		workingDir = os.Getenv("VSPHERE_WORKING_DIR")
		Expect(workingDir).NotTo(BeEmpty())

	})

	It("node unregister", func() {

		By("Get total Ready nodes")
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		Expect(nodeList.Items).NotTo(BeEmpty(), "Unable to find ready and schedulable Node")
		Expect(len(nodeList.Items) > 1).To(BeTrue(), "At least 2 nodes are required for this test")

		totalNodes := len(nodeList.Items)

		node1 := nodeList.Items[0]

		govMoMiClient, err := vsphere.GetgovmomiClient(nil)
		Expect(err).NotTo(HaveOccurred())

		finder := find.NewFinder(govMoMiClient.Client, true)
		ctx, _ := context.WithCancel(context.Background())

		vmPath := filepath.Join(workingDir, string(node1.ObjectMeta.Name))
		vm, err := finder.VirtualMachine(ctx, vmPath)
		Expect(err).NotTo(HaveOccurred())

		// Find .vmx file path to be used to re register the node
		var nodeVM mo.VirtualMachine
		err = vm.Properties(ctx, vm.Reference(), []string{"config.files"}, &nodeVM)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeVM.Config).NotTo(BeNil())

		vmFolder, err := finder.FolderOrDefault(ctx, workingDir)
		Expect(err).NotTo(HaveOccurred())
		host, err  := vm.HostSystem(ctx)
		Expect(err).NotTo(HaveOccurred())
		rpool ,err := vm.ResourcePool(ctx)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Power off the node: %v", node1.ObjectMeta.Name))
		_, err = vm.PowerOff(ctx)
		Expect(err).NotTo(HaveOccurred())
		err = vm.WaitForPowerState(ctx, vimtypes.VirtualMachinePowerStatePoweredOff)
		Expect(err).NotTo(HaveOccurred(), "Unable to power off the node")

		By(fmt.Sprintf("Unregister the node: %v", node1.ObjectMeta.Name))
		err = vm.Unregister(ctx)
		Expect(err).NotTo(HaveOccurred(), "Unable to unregister the node")

		// Ready nodes should be 1 less
		verifyReadyNodes(f.ClientSet, totalNodes - 1)

		By(fmt.Sprintf("Register the node: %v", node1.ObjectMeta.Name))
		registerTask, err := vmFolder.RegisterVM(ctx, nodeVM.Config.Files.VmPathName, node1.ObjectMeta.Name, false, rpool, host)
		Expect(err).NotTo(HaveOccurred())
		err = registerTask.Wait(ctx)
		Expect(err).NotTo(HaveOccurred())

		vm, err = finder.VirtualMachine(ctx, vmPath)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Power on the node: %v", node1.ObjectMeta.Name))
		vm.PowerOn(ctx)
		err = vm.WaitForPowerState(ctx, vimtypes.VirtualMachinePowerStatePoweredOn)
		Expect(err).NotTo(HaveOccurred(), "Unable to power on the node")

		// Ready nodes should be equal to earlier count
		verifyReadyNodes(f.ClientSet, totalNodes)


		// Sanity test that pod provisioning works
		scParameters := make(map[string]string)
		storagePolicy := os.Getenv("VSPHERE_SPBM_GOLD_POLICY")
		Expect(storagePolicy).NotTo(BeEmpty(), "Please set VSPHERE_SPBM_GOLD_POLICY system environment")
		scParameters[SpbmStoragePolicy] = storagePolicy
		invokeValidPolicyTest(f, client, namespace, scParameters)
	})
})

// verify ready status of nodes upto 1 minute
func verifyReadyNodes(client clientset.Interface, expectedNodes int){
	numNodes := 0
	for i := 0; i < 31; i++ {
		nodeList := framework.GetReadySchedulableNodesOrDie(client)
		Expect(nodeList.Items).NotTo(BeEmpty(), "Unable to find ready and schedulable Node")
		numNodes = len(nodeList.Items)
		if numNodes == expectedNodes {
			break
		}
		time.Sleep(5*time.Second)
	}

	Expect(numNodes).To(Equal(expectedNodes))
}