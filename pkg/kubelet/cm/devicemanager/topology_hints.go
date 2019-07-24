package devicemanager

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/socketmask"
)

// GetTopologyHints implements the TopologyManager HintProvider Interface which
// ensures the Device Manager is consulted when Topology Aware Hints for each
// container are created.
func (m *ManagerImpl) GetTopologyHints(pod v1.Pod, container v1.Container) map[string][]topologymanager.TopologyHint {
	deviceHints := make(map[string][]topologymanager.TopologyHint)

	for resourceObj, amountObj := range container.Resources.Limits {
		resource := string(resourceObj)
		amount := int(amountObj.Value())

		if m.isDevicePluginResource(resource) {
			klog.Infof("[devicemanager-topology] %v is a resource managed by device manager.", resource)

			if aligned := m.checkIfDeviceHasTopologyAlignment(resource); !aligned {
				klog.Infof("[devicemanager-topology] Device does not have a topology preference")
				deviceHints[resource] = nil
				continue
			}

			available := m.getAvailableDevices(resource)
			if available.Len() < amount {
				klog.Infof("[devicemanager-topology] Requested number of devices unavailable for %s. Requested: %d, Available: %d", resource, amount, available.Len())
				deviceHints[resource] = []topologymanager.TopologyHint{}
				continue
			}

			klog.Infof("[devicemanager-topology] Available devices for resource %v: %v", resource, available)

			deviceHints[resource] = m.generateDeviceTopologyHints(resource, available, amount)
		}
	}

	return deviceHints
}

func (m *ManagerImpl) checkIfDeviceHasTopologyAlignment(resource string) bool {
	// All devices must have Topology set for us to assume they care about alignment.
	for device := range m.allDevices[resource] {
		if m.allDevices[resource][device].Topology == nil {
			return false
		}
	}
	return true
}

func (m *ManagerImpl) getAvailableDevices(resource string) sets.String {
	// Gets Devices in use.
	m.updateAllocatedDevices(m.activePods())
	devicesInUse := m.allocatedDevices[resource]

	// Strip all devices in use from the list of healthy ones.
	return m.healthyDevices[resource].Difference(devicesInUse)
}

func (m *ManagerImpl) iterateDeviceCombinations(devices sets.String, amount int, callback func(sets.String)) {
	// Internal helper function to accumulate the combination before calling the callback.
	var iterate func(devices, accum []string)
	iterate = func(devices, accum []string) {
		// Base case: we have a combination of size 'amount'.
		if len(accum) == amount {
			callback(sets.NewString(accum...))
			return
		}
		for i := range devices {
			iterate(devices[i+1:], append(accum, devices[i]))
		}
	}
	iterate(devices.List(), []string{})
}

func (m *ManagerImpl) generateDeviceTopologyHints(resource string, devices sets.String, amount int) []topologymanager.TopologyHint {
	// Initialize minAffinity to a full affinity mask.
	minAffinity, _ := socketmask.NewSocketMask()
	minAffinity.Fill()

	// Iterate through all combinations of devices and build hints from them.
	hints := []topologymanager.TopologyHint{}
	m.iterateDeviceCombinations(devices, amount, func(combination sets.String) {
		// Compute the affinity of the device combination.
		affinity, _ := socketmask.NewSocketMask()
		for c := range combination {
			affinity.Add(int(m.allDevices[resource][c].Topology.Node.Id))
		}
		// Strip out duplicates
		for _, h := range hints {
			if h.SocketAffinity.IsEqual(affinity) {
				return
			}
		}
		// Update minAffinity if relevant
		if affinity.IsNarrowerThan(minAffinity) {
			minAffinity = affinity
		}
		// Create a new hint from the calculated affinity and add it to the list of hints.
		// We set all hint preferences to 'false' on the first pass through.
		hints = append(hints, topologymanager.TopologyHint{affinity, false})
	})

	// Loop back through all hints and update the 'Preferred' field based on
	// counting the number of bits sets in the affinity mask and comparing it
	// to the minAffinity. Only those with an equal number of bits set will be
	// considered preferred.
	for i := range hints {
		if hints[i].SocketAffinity.Count() == minAffinity.Count() {
			hints[i].Preferred = true
		}
	}

	return hints
}
