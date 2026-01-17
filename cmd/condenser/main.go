package main

import (
	httpapi "condenser/internal/api/http"
	"condenser/internal/env"
	"condenser/internal/monitor"
	"log"
	"net/http"
)

func main() {
	// == bootstrap ==
	bootstrap := env.NewBootstrapManager()
	if err := bootstrap.SetupRuntime(); err != nil {
		log.Fatal(err)
	}

	// == rest api ==
	// start Management Server
	managementAddr := "127.0.0.1:7755"
	managementRouter := httpapi.NewApiRouter()
	go func() {
		log.Printf("[*] management server listening on %s", managementAddr)
		if err := http.ListenAndServe(managementAddr, managementRouter); err != nil {
			log.Fatal(err)
		}
	}()

	hookAddr := ":7756"
	hookRouter := httpapi.NewHookRouter()
	// start Hook Server
	go func() {
		log.Printf("[*] hook server listening on %s", hookAddr)
		if err := http.ListenAndServe(hookAddr, hookRouter); err != nil {
			log.Fatal(err)
		}
	}()

	// start Swagger
	swaggerAddr := ":7757"
	swaggerRouter := httpapi.NewSwaggerRouter()
	go func() {
		log.Printf("[*] swagger listening on %s", swaggerAddr)
		if err := http.ListenAndServe(swaggerAddr, swaggerRouter); err != nil {
			log.Fatal(err)
		}
	}()

	// start monitoring
	log.Println("[*] Container Monitoring Start")
	containerMonitoring := monitor.NewContainerMonitor()
	containerMonitoring.Start()
}
