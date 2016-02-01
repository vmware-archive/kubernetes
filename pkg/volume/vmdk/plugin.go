/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package vmdk

 import (
         // VMware's client
	 "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/volume"
)

const (
        vmdkPluginName = "kubernetes.io/vmdk"
)

func ProbeVolumePlugins() []volume.VolumePlugin {
        return []volume.VolumePlugin{&vmdkPlugin{}}
}

type vmdkPlugin struct {
        host volume.VolumeHost
}

type vmdk struct {
        dataStore string
        vmdkVolumeID string
}

func (p *vmdkPlugin) Init(host volume.VolumeHost) {
        p.host = host
}

func (p vmdkPlugin) Name() string {
        return vmdkPluginName
}

func (p *vmdkPlugin) CanSupport(spec *volume.Spec) bool {
        return (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.VMDKVolume != nil) || (spec.Volume != nil && spec.Volume.VMDKVolume != nil)
}

func (p *vmdkPlugin) NewBuilder(spec *volume.Spec, pod *api.Pod, opts volume.VolumeOptions) (volume.Builder, error) {
        return nil, nil
}

func (p *vmdkPlugin) NewCleaner(datasetName string, podUID types.UID) (volume.Cleaner, error) {
        return nil, nil
}
