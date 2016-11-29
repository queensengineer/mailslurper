package main

import (
	"net/http"
	"os"
)

func setupAndStartAdminMux() {
	adminMux := http.NewServeMux()

	adminMux.Handle("/www/", http.StripPrefix("/www/", http.FileServer(http.Dir("./www/"))))
	adminMux.Handle("/", baseMiddleware(http.HandlerFunc(index)))
	adminMux.Handle("/admin", baseMiddleware(http.HandlerFunc(admin)))
	adminMux.Handle("/savedsearches", baseMiddleware(http.HandlerFunc(manageSavedSearches)))
	adminMux.Handle("/servicesettings", baseMiddleware(http.HandlerFunc(getServiceSettings)))
	adminMux.Handle("/version", baseMiddleware(http.HandlerFunc(getVersion)))
	adminMux.Handle("/masterversion", baseMiddleware(http.HandlerFunc(getVersionFromMaster)))

	go func() {
		if err := http.ListenAndServe(config.GetFullWWWBindingAddress(), adminMux); err != nil {
			logger.Fatalf("Error starting HTTP admin listener: %s", err.Error())
			os.Exit(-1)
		}
	}()
}
