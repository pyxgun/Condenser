package csm

import (
	"fmt"
	"time"
)

func NewCsmManager(csmStore *CsmStore) *CsmManager {
	return &CsmManager{
		csmStore: csmStore,
	}
}

type CsmManager struct {
	csmStore *CsmStore
}

func (m *CsmManager) StoreContainer(containerId string, state string, pid int) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		st.Containers[containerId] = ContainerInfo{
			ContainerId: containerId,
			State:       state,
			Pid:         pid,
			CreatedAt:   time.Now(),
		}
		return nil
	})
}

func (m *CsmManager) RemoveContainer(containerId string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		for id, c := range st.Containers {
			if c.ContainerId == containerId {
				delete(st.Containers, id)
				return nil
			}
		}
		return fmt.Errorf("containerId=%s not found", containerId)
	})
}

func (m *CsmManager) UpdateContainer(containerId string, state string, pid int) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		c, ok := st.Containers[containerId]
		if !ok {
			return fmt.Errorf("containerId=%s not found", containerId)
		}

		c.State = state
		if pid >= 0 {
			c.Pid = pid
		}
		st.Containers[containerId] = c
		return nil
	})
}
