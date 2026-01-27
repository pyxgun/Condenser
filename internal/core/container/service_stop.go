package container

import (
	"condenser/internal/runtime"
	"fmt"
)

// == service: stop ==
func (s *ContainerService) Stop(stopParameter ServiceStopModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(stopParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", stopParameter.ContainerId)
	}

	containerState, err := s.getContainerState(containerId)
	if err != nil {
		return "", err
	}

	switch containerState {
	case "running":
		// stop container
		if err := s.stopContainer(containerId); err != nil {
			return "", fmt.Errorf("stop failed: %w", err)
		}
	default:
		return "", fmt.Errorf("stop operation not allowed to current container status: %s", containerState)
	}
	return containerId, nil
}

func (s *ContainerService) stopContainer(containerId string) error {
	// runtime: stop
	if err := s.runtimeHandler.Stop(
		runtime.StopModel{
			ContainerId: containerId,
		},
	); err != nil {
		return err
	}
	return nil
}
