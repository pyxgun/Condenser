package hook

import (
	"condenser/internal/env"
	"condenser/internal/store/csm"
	"fmt"
)

func NewHookService() *HookService {
	return &HookService{
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(env.CsmStorePath)),
	}
}

type HookService struct {
	csmHandler csm.CsmHandler
}

func (s *HookService) UpdateCsm(stateParameter ServiceStateModel, eventType string) error {
	// switch eventType
	switch eventType {
	case "createRuntime":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm create failed: %w", err)
		}
	case "createContainer":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm update failed: %w", err)
		}
	case "poststart":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm update failed: %w", err)
		}
	case "stopContainer":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm update failed: %w", err)
		}
	case "poststop":
		if err := s.csmHandler.RemoveContainer(stateParameter.Id); err != nil {
			return fmt.Errorf("csm remove failed: %w", err)
		}
	default:
		return fmt.Errorf("csm unknown eventType: %s", eventType)
	}
	return nil
}
