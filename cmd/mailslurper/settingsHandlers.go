package main

import (
	"net/http"

	"github.com/adampresley/webframework/httpService"
	"github.com/mailslurper/mailslurper"
)

/*
getServiceSettings returns the settings necessary to talk to the MailSlurper
back-end service tier.
*/
func getServiceSettings(writer http.ResponseWriter, request *http.Request) {
	settings := mailslurper.ServiceSettings{
		ServiceAddress: config.ServiceAddress,
		ServicePort:    config.ServicePort,
		Version:        SERVER_VERSION,
	}

	httpService.WriteJSON(writer, settings, 200)
}

/*
getVersion outputs the current running version of this MailSlurper server instance
*/
func getVersion(writer http.ResponseWriter, request *http.Request) {
	result := mailslurper.Version{
		Version: SERVER_VERSION,
	}

	httpService.WriteJSON(writer, result, 200)
}

/*
getVersionFromMaster returns the current MailSlurper version from GitHub
*/
func getVersionFromMaster(writer http.ResponseWriter, request *http.Request) {
	var err error
	var result *mailslurper.Version

	if result, err = mailslurper.GetServerVersionFromMaster(); err != nil {
		logger.Errorf("Error getting version file from Github: %s", err.Error())
		httpService.WriteText(writer, "There was an error reading the version file from GitHub", 500)
		return
	}

	httpService.WriteJSON(writer, result, 200)
}
