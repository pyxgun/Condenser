package hook

import (
	"condenser/internal/core/container"
	"condenser/internal/core/policy"
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"fmt"
)

func NewHookService() *HookService {
	return &HookService{
		csmHandler:    csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		cgroupHandler: container.NewContaierService(),
		policyHandler: policy.NewwServicePolicy(),
	}
}

type HookService struct {
	csmHandler    csm.CsmHandler
	cgroupHandler container.CgroupServiceHandler
	policyHandler policy.PolicyServiceHandler
}

func (s *HookService) HookAction(stateParameter ServiceStateModel, eventType string) error {
	// switch eventType
	switch eventType {
	case "createRuntime":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm update failed: %w", err)
		}
	case "createContainer":
		if err := s.csmHandler.UpdateContainer(stateParameter.Id, stateParameter.Status, stateParameter.Pid); err != nil {
			return fmt.Errorf("csm update failed: %w", err)
		}
		// change cgroup dir mode: 755 -> 555
		if err := s.cgroupHandler.ChangeCgroupMode(stateParameter.Id); err != nil {
			return fmt.Errorf("chmod cgroup path failed: %w", err)
		}
		// commit policy
		if err := s.policyHandler.CommitPolicy(); err != nil {
			return fmt.Errorf("policy commit failed: %w", err)
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
		// commit policy
		if err := s.policyHandler.CommitPolicy(); err != nil {
			return fmt.Errorf("policy commit failed: %w", err)
		}
	default:
		return fmt.Errorf("csm unknown eventType: %s", eventType)
	}
	return nil
}
