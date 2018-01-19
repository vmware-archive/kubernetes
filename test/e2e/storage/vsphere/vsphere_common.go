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
	"os"
	"strconv"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	SPBMPolicyName            = "VSPHERE_SPBM_POLICY_NAME"
	StorageClassDatastoreName = "VSPHERE_DATASTORE"
	SecondSharedDatastore     = "VSPHERE_SECOND_SHARED_DATASTORE"
	LocalDatastore            = "VSPHERE_LOCAL_DATASTORE"
	KubernetesClusterName     = "VSPHERE_KUBERNETES_CLUSTER"
	SPBMTagPolicy             = "VSPHERE_SPBM_TAG_POLICY"
)

const (
	VCPClusterDatastore        = "CLUSTER_DATASTORE"
	SPBMPolicyDataStoreCluster = "VSPHERE_SPBM_POLICY_DS_CLUSTER"
)

const (
	VCPScaleVolumeCount   = "VCP_SCALE_VOLUME_COUNT"
	VCPScaleVolumesPerPod = "VCP_SCALE_VOLUME_PER_POD"
	VCPScaleInstances     = "VCP_SCALE_INSTANCES"
)

const (
	VCPStressInstances  = "VCP_STRESS_INSTANCES"
	VCPStressIterations = "VCP_STRESS_ITERATIONS"
)

const (
	VCPPerfVolumeCount   = "VCP_PERF_VOLUME_COUNT"
	VCPPerfVolumesPerPod = "VCP_PERF_VOLUME_PER_POD"
	VCPPerfIterations    = "VCP_PERF_ITERATIONS"
)

func GetAndExpectStringEnvVar(varName string) string {
	varValue := os.Getenv(varName)
	if varValue == "" {
		framework.Skipf("Environment variable " + varName + " is not set. Skipping the test.")
	}
	return varValue
}

func GetAndExpectIntEnvVar(varName string) int {
	varValue := GetAndExpectStringEnvVar(varName)
	varIntValue, err := strconv.Atoi(varValue)
	if err != nil {
		framework.Skipf("Error parsing " + varName + ", which is expected to be an integer value. Skipping the test.")
	}
	return varIntValue
}
