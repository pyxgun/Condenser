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

func (m *CsmManager) StoreContainer(containerId string, state string, pid int, repo, ref string, command []string, name string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		st.Containers[containerId] = ContainerInfo{
			ContainerId:   containerId,
			ContainerName: name,
			State:         state,
			Pid:           pid,
			Repository:    repo,
			Reference:     ref,
			Command:       command,
			CreatedAt:     time.Now(),
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

func (m *CsmManager) IsNameAlreadyUsed(name string) bool {
	var result bool
	_ = m.csmStore.withLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerName != name {
				continue
			}
			result = true
			return nil
		}
		result = false
		return nil
	})
	return result
}

func (m *CsmManager) GetContainerIdByName(name string) (string, error) {
	var containerId string
	err := m.csmStore.withLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerName != name {
				continue
			}
			containerId = c.ContainerId
			return nil
		}
		return fmt.Errorf("container: %s not found", name)
	})
	return containerId, err
}

func (m *CsmManager) GetContainerNameById(containerId string) (string, error) {
	var containerName string
	err := m.csmStore.withLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			containerName = c.ContainerName
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return containerName, err
}

func (m *CsmManager) ResolveContainerId(str string) (string, error) {
	var containerId string
	// 1. resolve container id by name
	containerId, err := m.GetContainerIdByName(str)
	if err != nil { // the string is not containerId
		// 2. check container exist by id
		if _, err := m.GetContainerById(str); err != nil {
			// the string is not a exist container
			return "", err
		}
		containerId = str
	}
	return containerId, nil
}
