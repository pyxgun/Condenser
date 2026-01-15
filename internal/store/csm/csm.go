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

func (m *CsmManager) StoreContainer(containerId string, state string, pid int, repo, ref string, command []string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		st.Containers[containerId] = ContainerInfo{
			ContainerId: containerId,
			State:       state,
			Pid:         pid,
			Repository:  repo,
			Reference:   ref,
			Command:     command,
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
		switch state {
		case "creating":
			c.CreatingAt = time.Now()
		case "created":
			c.CreatedAt = time.Now()
		case "running":
			c.StartedAt = time.Now()
		case "stopped":
			c.StoppedAt = time.Now()
		}

		if pid >= 0 {
			c.Pid = pid
		}
		st.Containers[containerId] = c
		return nil
	})
}

func (m *CsmManager) GetContainerList() ([]ContainerInfo, error) {
	var containerList []ContainerInfo
	err := m.csmStore.withLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			containerList = append(containerList, c)
		}
		return nil
	})
	return containerList, err
}

func (m *CsmManager) GetContainerById(containerId string) (ContainerInfo, error) {
	var containerInfo ContainerInfo
	err := m.csmStore.withLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			containerInfo = c
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return containerInfo, err
}
