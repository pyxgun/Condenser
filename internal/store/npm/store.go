package npm

import (
	"condenser/internal/utils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func NewNpmStore(path string) *NpmStore {
	return &NpmStore{
		path:              path,
		filesystemHandler: &utils.FilesystemExecutor{},
	}
}

type NpmStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *NpmStore) withLock(fn func(np *NetworkPolicy) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return s.atomicSave(st)
}

func (s *NpmStore) loadOrInit() (*NetworkPolicy, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			// nettwork policy file not exist
			return &NetworkPolicy{
				Version: "v1",
				DefaultRule: DefaultPolicy{
					EastWest: PolicyMode{
						Mode:    "deny_by_default",
						Logging: true,
					},
					NorthSouth: PolicyMode{
						Mode:    "observe",
						Logging: true,
					},
				},
				Policies: PolicyChain{
					EastWestPolicy:          []Policy{},
					NorthSouthObservePolicy: []Policy{},
					NorthSouthEnforcePolicy: []Policy{},
				},
			}, nil
		}
		return nil, err
	}

	var np NetworkPolicy
	if err := json.Unmarshal(b, &np); err != nil {
		return nil, fmt.Errorf("network policy json broken: %w", err)
	}
	return &np, nil
}

func (s *NpmStore) atomicSave(np *NetworkPolicy) error {
	tmp := s.path + ".tmp"

	b, err := json.MarshalIndent(np, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := s.filesystemHandler.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.filesystemHandler.Rename(tmp, s.path)
}

func (s *NpmStore) SetNetworkPolicy() error {
	return s.withLock(func(np *NetworkPolicy) error {
		np.Version = "v1"
		return nil
	})
}
