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
 
package numamanager
import (
 	"strconv"
 	"strings"
 	"github.com/golang/glog"
 	"k8s.io/api/core/v1"
 	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)
 
type NumaManager interface {
 	lifecycle.PodAdmitHandler
 	AddHintProvider(HintProvider)
 	RemovePod(podName string)
 	Store
 }
 
type numaManager struct {
 	//The list of componenets registered with the NumaManager
 	hintProviders []HintProvider
 	//List of Containers and their NUMA Allocations
 	podNUMAHints map[string]containers	
}
 
//Interface to be implemented by Numa Allocators 
type HintProvider interface {
 	GetNUMAHints(pod v1.Pod, container v1.Container) NumaMask
}
 
type Store interface {
	GetAffinity(pod v1.Pod, containerName string) NumaMask
}
 
type NumaMask struct {
 	Mask []int64
	Affinity bool
}
 
type containers map[string]NumaMask
var _ NumaManager = &numaManager{}
func NewNumaManager() NumaManager {
 	glog.Infof("[numamanager] Creating numa mananager")
 	var hp []HintProvider
 	pnh := make (map[string]containers)
 	numaManager := &numaManager{
 		hintProviders: hp,
 		podNUMAHints: pnh,
 	}
 	
	return numaManager
}

func (m *numaManager) GetAffinity(pod v1.Pod, containerName string) NumaMask {
 	return m.podNUMAHints[string(pod.UID)][containerName]
}

func (m *numaManager) calculateNUMAAffinity(pod v1.Pod, container v1.Container) NumaMask {
 	podNumaMask := NumaMask {
 		Mask:		nil,
 		Affinity:	false,
 	}
 	var numaResult []int64
 	numaResult = append(numaResult, 11)
 	for _, hp := range m.hintProviders {
 		numaMask := hp.GetNUMAHints(pod, container)
 		if numaMask.Affinity {
			podNumaMask.Affinity = true
 			orderedMask := getBitCount(numaMask.Mask)
			glog.Infof("[numamanager] Numa Affinity. Mask: %v Affinity: %v", orderedMask, numaMask.Affinity)
 			if numaResult = getNumaAffinity(orderedMask, numaResult); numaResult == nil {
 				glog.Infof("[numamanager] NO Numa Affinity. Result %v", numaResult)
 				break;
 			}
 		}
 		
	}
 	podNumaMask.Mask = append(podNumaMask.Mask, numaResult[0])
 	return podNumaMask
 	
}

func getBitCount(mask []int64) []int64 {
 	var orderedMask []int64
 	prevCount := 0
 	for _, bitset := range mask {
 		sbitset := strconv.Itoa(int(bitset))
 		count := 0
 		for _, bit := range sbitset {
 			if strings.Compare(string(bit), "1") == 0 {
 				count += 1
 			}
 		}
 		if count > prevCount {
 			orderedMask = append(orderedMask, bitset)
 			prevCount = count
 		}else {
 			orderedMask = append([]int64{bitset}, orderedMask...)
 		}
 	}
 	return orderedMask
}

func getNumaAffinity (maska, maskb []int64) []int64 {
 	var newResult []int64
 	for _, mask := range maska {
 		for _, resultMask := range maskb {
 			var affinity int64
 			affinity = 0
 			affinity = mask & resultMask 
 			if affinity != 0 {
 				newResult = append(newResult, affinity)
 			}
 		}
 	}
 	return newResult
}

func (m *numaManager) AddHintProvider(h HintProvider) {
 	m.hintProviders = append(m.hintProviders, h)
}

func (m *numaManager) RemovePod (podName string) {
 	glog.Infof("Remove pod func")
}

func (m *numaManager) Admit(attrs *lifecycle.PodAdmitAttributes) lifecycle.PodAdmitResult {
 	glog.Infof("[numamanager] NUMA Admit Handler")
 	pod := attrs.Pod
 	c := make (containers)
 	
	glog.Infof("[numamanager] Pod QoS Level: %v", pod.Status.QOSClass)
	
	qosClass := pod.Status.QOSClass
	
	if qosClass == "Guaranteed" {
		for _, container := range pod.Spec.Containers {
			result := m.calculateNUMAAffinity(*pod, container)
			if result.Affinity && result.Mask == nil {
					return lifecycle.PodAdmitResult{
					Admit:   false,
					Reason:	 "Numa Affinity Error",
					Message: "Resources cannot be allocated with Numa Locality",
				}

			}
			c[container.Name] = result		
		}
	
		m.podNUMAHints[string(pod.UID)] = c
		glog.Infof("[numamanager] NUMA Affinity for Pod: %v are %v", pod.UID, m.podNUMAHints[string(pod.UID)])
	
	} else {
		glog.Infof("[numamanager] NUMA Manager only affinitises Guaranteed pods.")
	}
	
	return lifecycle.PodAdmitResult{
		Admit:   true,
	}
}
