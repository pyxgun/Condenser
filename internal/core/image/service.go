package image

import (
	"condenser/internal/env"
	"condenser/internal/registry"
	"condenser/internal/registry/dockerhub"
	"condenser/internal/store/ilm"
	"condenser/internal/utils"
	"encoding/json"
	"errors"
	"strings"
)

func NewImageService() *ImageService {
	return &ImageService{
		filesystemHandler: utils.NewFilesystemExecutor(),
		registryHandler:   dockerhub.NewRegistryDockerHub(),
		ilmStoreHandler:   ilm.NewIlmStore(env.IlmStorePath),
		ilmHandler:        ilm.NewIlmManager(ilm.NewIlmStore(env.IlmStorePath)),
	}
}

type ImageService struct {
	filesystemHandler utils.FilesystemHandler
	registryHandler   registry.RegistryHandler
	ilmStoreHandler   ilm.IlmStoreHandler
	ilmHandler        ilm.IlmHandler
}

func (s *ImageService) Pull(pullParameter ServicePullModel) error {
	// pull image
	repository, reference, bundlePath, configPath, rootfsPath, err := s.registryHandler.PullImage(
		registry.RegistryPullModel{
			Image: pullParameter.Image,
			Os:    pullParameter.Os,
			Arch:  pullParameter.Arch,
		},
	)
	if err != nil {
		return err
	}

	// add ilm entry
	if err := s.ilmHandler.StoreImage(
		repository, reference,
		bundlePath, configPath, rootfsPath,
	); err != nil {
		return err
	}

	return nil
}

func (s *ImageService) Remove(removeParameter ServiceRemoveModel) error {
	repo, ref, err := s.parseImageRef(removeParameter.Image)
	if err != nil {
		return err
	}

	// remove directory
	bundlePath, err := s.ilmStoreHandler.GetBundlePath(repo, ref)
	if err != nil {
		return err
	}
	if err := s.filesystemHandler.RemoveAll(bundlePath); err != nil {
		return err
	}

	// remove ilm entry
	if err := s.ilmHandler.RemoveImage(repo, ref); err != nil {
		return err
	}

	return nil
}

func (s *ImageService) parseImageRef(imageStr string) (repository, reference string, err error) {
	// image string pattern
	// - ubuntu 				-> library/ubuntu:latest
	// - ubuntu:24.04 			-> library/ubuntu:24.04
	// - library/ubuntu:24.04 	-> library/ubuntu:24.04
	// - nginx@sha256:... 		-> library/nginx@sha256:...

	var repo, ref string
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		repo, ref = parts[0], parts[1]
	} else {
		parts := strings.SplitN(imageStr, ":", 2)
		repo = parts[0]
		if len(parts) == 2 && parts[1] != "" {
			ref = parts[1]
		} else {
			ref = "latest"
		}
	}

	if repo == "" {
		return "", "", errors.New("empty repository")
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	return repo, ref, nil
}

func (s *ImageService) GetImageConfig(filepath string) (ImageConfigFile, error) {
	b, err := s.filesystemHandler.ReadFile(filepath)
	if err != nil {
		return ImageConfigFile{}, err
	}

	var imageConfig ImageConfigFile
	if err := json.Unmarshal(b, &imageConfig); err != nil {
		return ImageConfigFile{}, err
	}
	return imageConfig, nil
}
