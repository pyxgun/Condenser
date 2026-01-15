package http

import (
	_ "condenser/docs"

	containerHandler "condenser/internal/api/http/container"
	hookHandler "condenser/internal/api/http/hook"
	imageHandler "condenser/internal/api/http/image"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title Condenser API
// @version 1.0
// @description High-level container runtime API for Raind stack
// @BasePath /
// @schemes http

func NewApiRouter() *chi.Mux {
	r := chi.NewRouter()
	containerHandler := containerHandler.NewRequestHandler()
	imageHandler := imageHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// == swagger ==
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	// == v1 ==
	// == containers ==
	r.Get("/v1/containers", containerHandler.GetContainerList)                                // get container list
	r.Get("/v1/containers/{containerId}", containerHandler.GetContainerById)                  // get container status by id
	r.Post("/v1/containers", containerHandler.CreateContainer)                                // create container
	r.Post("/v1/containers/{containerId}/actions/start", containerHandler.StartContainer)     // start container
	r.Post("/v1/containers/{containerId}/actions/stop", containerHandler.StopContainer)       // stop container
	r.Post("/v1/containers/{containerId}/actions/exec", containerHandler.ExecContainer)       // exec container
	r.Delete("/v1/containers/{containerId}/actions/delete", containerHandler.DeleteContainer) // delete container

	// == images ==
	r.Get("/v1/images", imageHandler.GetImageList)   // get image list
	r.Post("/v1/images", imageHandler.PullImage)     // pull image
	r.Delete("/v1/images", imageHandler.RemoveImage) // remove image

	return r
}

func NewHookRouter() *chi.Mux {
	r := chi.NewRouter()
	hookHandler := hookHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// == hook ==
	r.Post("/v1/hooks/droplet", hookHandler.ApplyHook)

	return r
}
