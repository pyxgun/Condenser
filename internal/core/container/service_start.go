package container

import (
	"condenser/internal/runtime"
	"fmt"
)

// == service: start ==
func (s *ContainerService) Start(startParameter ServiceStartModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(startParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", startParameter.ContainerId)
	}

	containerState, err := s.getContainerState(containerId)
	if err != nil {
		return "", err
	}

	switch containerState {
	case "created":
		// start container
		if err := s.startContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	case "running":
		// already started. ignore operation
		return "", fmt.Errorf("container: %s already started", containerId)

	case "stopped":
		// create container
		if err := s.createContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}
		// start container
		if err := s.startContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	default:
		return "", fmt.Errorf("start operation not allowed to current container status: %s", containerState)
	}

	return containerId, nil
}

func (s *ContainerService) startContainer(containerId string, tty bool) error {
	// runtime: start
	if err := s.runtimeHandler.Start(
		runtime.StartModel{
			ContainerId: containerId,
			Tty:         tty,
		},
	); err != nil {
		return err
	}

	return nil
}
