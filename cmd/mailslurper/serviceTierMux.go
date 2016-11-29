package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mailslurper/mailslurper"
)

func setupAndStartServiceTierMux() {
	var err error

	serviceTierConfig = &mailslurper.ServiceTierConfiguration{
		Address:  config.ServiceAddress,
		Port:     config.ServicePort,
		Database: database,
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
	}

	serviceMux := http.NewServeMux()

	serviceMux.Handle("/version", baseMiddleware(http.HandlerFunc(version)))
	serviceMux.Handle("/mail", baseMiddleware(http.HandlerFunc(getMailCollection)))
	serviceMux.Handle("/mail/", baseMiddleware(http.HandlerFunc(mailEndpoint)))
	serviceMux.Handle("/mailcount", baseMiddleware(http.HandlerFunc(getMailCount)))
	serviceMux.Handle("/pruneoptions", baseMiddleware(http.HandlerFunc(getPruneOptions)))

	/*
		AddRoute("/mail/{mailID}", controllers.GetMail, "GET", "OPTIONS").
		AddRoute("/mail/{mailID}/message", controllers.GetMailMessage, "GET", "OPTIONS").
		AddRoute("/mail/{mailID}/attachment/{attachmentID}", controllers.DownloadAttachment, "GET", "OPTIONS").
		AddRoute("/mail", controllers.GetMailCollection, "GET", "OPTIONS").
		AddRoute("/mail", controllers.DeleteMail, "DELETE", "OPTIONS").
		AddRoute("/mailcount", controllers.GetMailCount, "GET", "OPTIONS").
		AddRoute("/pruneoptions", controllers.GetPruneOptions, "GET", "OPTIONS")
	*/

	if serviceTierConfig.CertFile != "" && serviceTierConfig.KeyFile != "" {
		err = http.ListenAndServeTLS(config.GetFullServiceAppAddress(), serviceTierConfig.CertFile, serviceTierConfig.KeyFile, serviceMux)
	} else {
		err = http.ListenAndServe(config.GetFullServiceAppAddress(), serviceMux)
	}

	if err != nil {
		log.Fatalf("Error starting the MailSlurper service server: %s", err.Error())
		os.Exit(-1)
	}
}
