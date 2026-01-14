package ilm

import (
	"fmt"
	"time"
)

func NewIlmManager(ilmStore *IlmStore) *IlmManager {
	return &IlmManager{
		ilmStore: ilmStore,
	}
}

type IlmManager struct {
	ilmStore *IlmStore
}

func (m *IlmManager) StoreImage(repository, reference, bundlePath, configPath, rootfsPath string) error {
	return m.ilmStore.withLock(func(st *ImageLayerState) error {
		if st.Repositories == nil {
			st.Repositories = map[string]RepositoryInfo{}
		}
		repoInfo, ok := st.Repositories[repository]
		if !ok {
			repoInfo = RepositoryInfo{
				References: map[string]ReferenceInfo{},
			}
		}
		if repoInfo.References == nil {
			repoInfo.References = map[string]ReferenceInfo{}
		}

		repoInfo.References[reference] = ReferenceInfo{
			BundlePath: bundlePath,
			ConfigPath: configPath,
			RootfsPath: rootfsPath,
			CreatedAt:  time.Now(),
		}

		st.Repositories[repository] = repoInfo
		return nil
	})
}

func (m *IlmManager) RemoveImage(repository string, reference string) error {
	return m.ilmStore.withLock(func(st *ImageLayerState) error {
		repo, ok := st.Repositories[repository]
		if !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		if _, ok := repo.References[reference]; !ok {
			return fmt.Errorf("%s:%s not found", repository, reference)
		}
		delete(st.Repositories[repository].References, reference)
		return nil
	})
}
