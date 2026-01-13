package main

import (
	httpapi "condenser/internal/api/http"
	"condenser/internal/env"
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
	// public api
	publicAddr := ":7755"
	publicRouter := httpapi.NewApiRouter()

	// hook (localhost only)
	hookAddr := ":7756"
	hookRouter := httpapi.NewHookRouter()

	// execute router
	go func() {
		log.Printf("[*] hook server listening on %s", hookAddr)
		if err := http.ListenAndServe(hookAddr, hookRouter); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("[*] api server listening on %s", publicAddr)
	if err := http.ListenAndServe(publicAddr, publicRouter); err != nil {
		log.Fatal(err)
	}
}
