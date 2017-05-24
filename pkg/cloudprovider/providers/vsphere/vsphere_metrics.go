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
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

const (
	request_createvolume                      = "CreateVolume"
	request_createvolume_with_policy          = "CreateVolumeWithPolicy"
	request_createvolume_with_raw_vsan_policy = "CreateVolumeWithRawVSANPolicy"
	request_deletevolume                      = "DeleteVolume"
	request_attachvolume                      = "AttachVolume"
	request_detachvolume                      = "DetachVolume"
	request_diskIsAttached                    = "DiskIsAttached"
	request_disksAreAttached                  = "DisksAreAttached"
)

var vsphereApiMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "cloudprovider_vsphere_api_request_duration_seconds",
		Help: "Latency of vsphere api call",
	},
	[]string{"request"},
)

var vsphereApiErrorMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cloudprovider_vsphere_api_request_errors",
		Help: "vsphere Api errors",
	},
	[]string{"request"},
)

func registerMetrics() {
	prometheus.MustRegister(vsphereApiMetric)
	prometheus.MustRegister(vsphereApiErrorMetric)
}

func recordvSphereMetric(actionName string, requestTime time.Time, err error) {
	var timeTaken float64
	if !requestTime.IsZero() {
		timeTaken = time.Since(requestTime).Seconds()
	} else {
		timeTaken = 0
	}
	if err != nil {
		vsphereApiErrorMetric.With(prometheus.Labels{"request": actionName}).Inc()
	} else {
		vsphereApiMetric.With(prometheus.Labels{"request": actionName}).Observe(timeTaken)
	}
}

func recordCreateVolumeMetric(volumeOptions *VolumeOptions, requestTime time.Time, err error) {
	var actionName string
	if volumeOptions.StoragePolicyName != "" {
		actionName = request_createvolume_with_policy
	} else if volumeOptions.VSANStorageProfileData != "" {
		actionName = request_createvolume_with_raw_vsan_policy
	} else {
		actionName = request_createvolume
	}
	var timeTaken float64
	if !requestTime.IsZero() {
		timeTaken = time.Since(requestTime).Seconds()
	} else {
		timeTaken = 0
	}
	if err != nil {
		vsphereApiErrorMetric.With(prometheus.Labels{"request": actionName}).Inc()
	} else {
		vsphereApiMetric.With(prometheus.Labels{"request": actionName}).Observe(timeTaken)
	}
}
