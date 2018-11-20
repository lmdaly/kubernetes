/*Copyright 2015 The Kubernetes Authors.
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
 
package topologymanager
import (
 	"github.com/golang/glog"	
 	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)
 
type fakeManager struct{}
 
func NewFakeManager() Manager {
        glog.Infof("[fake topologymanager] NewFakeManager")
 	return &fakeManager{}
}

func (m *fakeManager) GetAffinity(podUID string, containerName string) TopologyHints {
	glog.Infof("[fake topologymanager] GetAffinity podUID: %v container name:  %v", podUID, containerName)
 	return TopologyHints{}
}

func (m *fakeManager) AddHintProvider(h HintProvider) {
	glog.Infof("[fake topologymanager] AddHintProvider HintProvider:  %v", h)
}

func (m *fakeManager) AddPod(pod *v1.Pod, containerID string) error {
	glog.Infof("[fake topologymanager] AddPod  pod: %v container id:  %v", pod, containerID)
	return nil
}

func (m *fakeManager) RemovePod (containerID string) error {
	glog.Infof("[fake topologymanager] RemovePod container id:  %v", containerID)
	return nil 
}

func (m *fakeManager) Admit(attrs *lifecycle.PodAdmitAttributes) lifecycle.PodAdmitResult {
 	glog.Infof("[fake topologymanager] Topology Admit Handler")
	return lifecycle.PodAdmitResult{
		Admit:	true,
	} 
}
