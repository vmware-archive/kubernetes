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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	manifestPath     = "test/e2e/testing-manifests/statefulset/nginx"
	mountPath        = "/usr/share/nginx/html"
	storageclassname = "nginx-sc"
)

var _ = SIGDescribe("vsphere statefulsets", func() {
	f := framework.NewDefaultFramework("vsphere-statefulsets")
	var (
		namespace string
		client    clientset.Interface
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		namespace = f.Namespace.Name
		client = f.ClientSet
	})
	AfterEach(func() {
		framework.Logf("Deleting all statefulset in namespace: %v", namespace)
		framework.DeleteAllStatefulSets(client, namespace)
	})

	It("vsphere statefulsets testing", func() {
		By("Creating StorageClass for Statefulsets")
		scParameters := make(map[string]string)
		scParameters["diskformat"] = "thin"
		scSpec := getVSphereStorageClassSpec(storageclassname, scParameters)
		sc, err := client.StorageV1().StorageClasses().Create(scSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(sc.Name, nil)

		By("Creating statefulsets with number of Replica :4")
		statefulsetsTester := framework.NewStatefulSetTester(client)
		statefulsets := statefulsetsTester.CreateStatefulSet(manifestPath, namespace)
		statefulsetsTester.CheckMount(statefulsets, mountPath)

		By("Scaling down statefulsets to number of Replica: 2")
		statefulsetsTester.Scale(statefulsets, 2)
		statefulsetsTester.WaitForStatusReplicas(statefulsets, 2)

		By("Scaling up statefulsets to number of Replica: 5")
		statefulsetsTester.Scale(statefulsets, 5)
		statefulsetsTester.WaitForStatusReplicas(statefulsets, 5)
	})
})
