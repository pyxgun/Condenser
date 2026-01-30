package http

import (
	_ "condenser/docs"
	"log"
	"net/http"
	"os"
	"strings"

	certHandler "condenser/internal/api/http/cert"
	containerHandler "condenser/internal/api/http/container"
	hookHandler "condenser/internal/api/http/hook"
	imageHandler "condenser/internal/api/http/image"
	"condenser/internal/api/http/logger"
	logHandler "condenser/internal/api/http/logs"
	policyHandler "condenser/internal/api/http/policy"
	websocketHandler "condenser/internal/api/http/websocket"
	"condenser/internal/utils"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title Condenser API
// @version 1.0
// @description High-level container runtime API for Raind stack
// @BasePath /
// @schemes http

func NewSwaggerRouter() *chi.Mux {
	r := chi.NewRouter()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// == swagger ==
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	return r
}

func NewApiRouter() *chi.Mux {
	r := chi.NewRouter()
	containerHandler := containerHandler.NewRequestHandler()
	imageHandler := imageHandler.NewRequestHandler()
	socketHandler := websocketHandler.NewRequestHandler()
	execSocketHandler := websocketHandler.NewExecRequestHandler()
	policyHandler := policyHandler.NewRequestHandler()
	logHandler := logHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/cli/"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

	// == v1 ==
	// == containers ==
	r.Get("/v1/containers", containerHandler.GetContainerList)                                // get container list
	r.Get("/v1/containers/{containerId}", containerHandler.GetContainerById)                  // get container status by id
	r.Get("/v1/containers/{containerId}/log", containerHandler.GetContainerLog)               // get container log
	r.Post("/v1/containers", containerHandler.CreateContainer)                                // create container
	r.Post("/v1/containers/{containerId}/actions/start", containerHandler.StartContainer)     // start container
	r.Post("/v1/containers/{containerId}/actions/stop", containerHandler.StopContainer)       // stop container
	r.Post("/v1/containers/{containerId}/actions/exec", containerHandler.ExecContainer)       // exec container
	r.Delete("/v1/containers/{containerId}/actions/delete", containerHandler.DeleteContainer) // delete container

	// == images ==
	r.Get("/v1/images", imageHandler.GetImageList)   // get image list
	r.Post("/v1/images", imageHandler.PullImage)     // pull image
	r.Delete("/v1/images", imageHandler.RemoveImage) // remove image

	// == websocket ==
	r.Get("/v1/containers/{containerId}/attach", socketHandler.ServeHTTP)
	r.Get("/v1/containers/{containerId}/exec/attach", execSocketHandler.ServeHTTP)

	// == policy ==
	r.Get("/v1/policies/{chain}", policyHandler.GetPolicyList)      // get policy
	r.Post("/v1/policies", policyHandler.AddPolicy)                 // add policy
	r.Post("/v1/policies/commit", policyHandler.CommitPolicy)       // commit policy
	r.Post("/v1/policies/revert", policyHandler.RevertPolicy)       // revert policy
	r.Post("/v1/policies/ns/mode", policyHandler.ChangeNSMode)      // change NS mode
	r.Delete("/v1/policies/{policyId}", policyHandler.RemovePolicy) // remove policy

	// == logs ==
	r.Get("/v1/logs/netflow", logHandler.GetNetflowLog) // get netflow log

	return r
}

func NewHookRouter() *chi.Mux {
	r := chi.NewRouter()
	hookHandler := hookHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/container"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

	// == hook ==
	r.Post("/v1/hooks/droplet", hookHandler.ApplyHook)

	return r
}

func NewCARouter() *chi.Mux {
	r := chi.NewRouter()
	certHandler := certHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/droplet/"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

	// == CA ==
	r.Post("/v1/pki/sign", certHandler.SignCSRHandler)

	return r
}

func RequireSPIFFE(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
			// validate
			cert := r.TLS.PeerCertificates[0]
			spiffeId := cert.URIs[0]
			if strings.HasPrefix(spiffeId.String(), prefix) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

func openAuditLog() *os.File {
	fd, err := os.OpenFile(utils.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatal("open audit log file failed")
	}
	return fd
}
