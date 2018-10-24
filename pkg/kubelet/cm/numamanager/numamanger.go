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
	"bytes"
	"math"
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
 	//The list of components registered with the NumaManager
 	hintProviders []HintProvider
 	//List of Containers and their NUMA Allocations
 	podNUMAHints map[string]containers	
}
 
//Interface to be implemented by Numa Allocators 
type HintProvider interface {
    	GetNUMAHints(resource string, amount int) NumaMask
}
 
type Store interface {
	GetAffinity(podUID string, containerName string) NumaMask
}
 
type NumaMask struct {
	Mask [][]int64
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

func (m *numaManager) GetAffinity(podUID string, containerName string) NumaMask {
 	return m.podNUMAHints[podUID][containerName]
}

func (m *numaManager) calculateNUMAAffinity(pod v1.Pod, container v1.Container) NumaMask {
        var row,col int = 1, 1
        nmTest := make([][]int64, row)
        for i:= range nmTest {
                nmTest[i] = make([]int64,col)
        }

	podNumaMask := NumaMask {
                Mask:           nmTest,
                Affinity:       false,
        }
        
	var maskHolder []string
	count := 0 
	var finalMask int64
        for _, hp := range m.hintProviders {
		for resource, amount := range container.Resources.Requests {
			glog.Infof("Container Resource Name in NUMA Manager: %v, Amount: %v", resource, amount.Value())
			numaMask := hp.GetNUMAHints(string(resource), int(amount.Value()))                          
			if numaMask.Affinity && numaMask.Mask != nil {
				if count == 0 {
					outerLen := len(numaMask.Mask)
					var innerLen int = 0
					for i := 0; i < outerLen; i++ {
						if innerLen < len(numaMask.Mask[i]) {
							innerLen = len(numaMask.Mask[i])
						}
					}
					var buffer bytes.Buffer
					var i, j int = 0, 0
					for i = 0; i < outerLen; i++ {
						for j = 0; j < innerLen; j++ {
        						buffer.WriteString("1")
    						}
						maskHolder = append(maskHolder, buffer.String())
						buffer.Reset()
					}
					count++
				}
				glog.Infof("[numa manager] MaskHolder : %v", maskHolder)
				//Arrange int array into array of strings 
				glog.Infof("[numa manager] %v is passed into arrange function",numaMask.Mask)   
				arrangedMask := arrangeMask(numaMask.Mask)						
				newMask := getNUMAAffinity(arrangedMask, maskHolder)
				//FIXME - take most suitable value from mask, not just random
				glog.Infof("[numa manager] New Mask after getNUMAAffinity (new mask) : %v ",newMask)
				finalMask = parseMask(newMask)
				glog.Infof("[numamanager] Mask Int64 (finalMask): %v", finalMask)
				maskHolder = newMask
				glog.Infof("[numa manager] New MaskHolder: %v", maskHolder) 
     
			} else if numaMask.Affinity && numaMask.Mask == nil {
				glog.Infof("[numamanager] NO Numa Affinity.")
				return NumaMask {
					Mask:		nil,
					Affinity:   	true,   
				}       
			}  
		}
	}
	podNumaMask.Mask[0][0] = finalMask
	return podNumaMask       
}

func getNUMAAffinity(arrangedMask, maskHolder []string) []string {
	var numaTest []string
        for i:= 0; i < (len(maskHolder)); i++ {
        	for j:= 0; j < (len(arrangedMask)); j++ {
               		tempStr := andOperation(maskHolder[i],arrangedMask[j])
                      	if strings.Contains(tempStr, "1") {
                               	numaTest = append(numaTest, tempStr )
                      	}
               	}
     	}
        duplicates := map[string]bool{}
        for v:= range numaTest {
        	duplicates[numaTest[v]] = true
        }
       	// Place all keys from the map into a slice.
       	numaResult := []string{}
      	for key, _ := range duplicates {
       		numaResult = append(numaResult, key)
     	}
	
	return numaResult
}

func parseMask(mask []string) int64 {
	maskLen := len(mask)
	var maskStr string
	for i := 0; i < maskLen; i++ {
		if strings.Contains( mask[i], "1") {
			maskStr = mask[i]
			break
		} else {
			maskStr = "0"
		}
	}
	maskInt, _ := strconv.Atoi(maskStr)
	var maskInt64 int64
	maskInt64 = int64(maskInt)
	return maskInt64
}

func arrangeMask(mask [][]int64) []string {
	var socketStr []string
	var bufferNew bytes.Buffer
	outerLen := len(mask)
	innerLen := len(mask[0])
	for i := 0; i < outerLen; i++ {
		for j := 0; j < innerLen; j++ {
			if mask[i][j] == 1 {
				bufferNew.WriteString("1")
			} else if mask[i][j] == 0 {
				bufferNew.WriteString("0")
			}
		}
		socketStr = append(socketStr, bufferNew.String())
		bufferNew.Reset()
	}
	return socketStr
}

func andOperation(val1, val2 string) (string) {
	l1, l2 := len(val1), len(val2)
	//compare lengths of strings - pad shortest with trailing zeros
	if l1 != l2 {
		// Get the bit difference
		var num int
		diff := math.Abs(float64(l1) - float64(l2))
		num = int(diff)
		if l1 < l2 {
			val1 = val1 + strings.Repeat("0", num)
		} else {
			val2 = val2 + strings.Repeat("0", num)
		}
	}
	length := len(val1)
	byteArr := make([]byte, length)
	for i := 0; i < length ; i++ {
		byteArr[i] = (val1[i] & val2[i])
    	}
	var finalStr string
	finalStr = string(byteArr[:])	
	
	return finalStr
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
