// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

//go:generate esc -o ./www/www.go -pkg www -ignore DS_Store|README\.md|LICENSE -prefix /www/ ./www

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/adampresley/presleylife/www"
	"github.com/adampresley/webframework/console"
	"github.com/adampresley/webframework/logging"
	"github.com/alecthomas/template"
	"github.com/mailslurper/mailslurper"
	"github.com/skratchdot/open-golang/open"
)

const (
	// Version of the MailSlurper Server application
	SERVER_VERSION string = "1.11.1"

	// Set to true while developing
	DEBUG_ASSETS bool = true

	CONFIGURATION_FILE_NAME string = "config.json"
)

var config *mailslurper.Configuration
var database mailslurper.IStorage
var logger *logging.Logger
var serviceTierConfig *mailslurper.ServiceTierConfiguration

func main() {
	var err error

	logger = logging.NewLoggerWithMinimumLevel("MailSlurper", logging.StringToLogType("debug"))
	logger.Infof("Starting MailSlurper Server v%s", SERVER_VERSION)

	/*
	 * Prepare SIGINT handler (CTRL+C)
	 */
	console.ListenForSIGINT(func() {
		logger.Infof("Shutting down")
		os.Exit(0)
	})

	/*
	 * Load configuration
	 */
	if config, err = mailslurper.LoadConfigurationFromFile(CONFIGURATION_FILE_NAME); err != nil {
		logger.Fatalf("There was an error reading the configuration file '%s': %s", CONFIGURATION_FILE_NAME, err.Error())
		os.Exit(-1)
	}

	/*
	 * Setup global database connection handle
	 */
	storageType, databaseConnection := config.GetDatabaseConfiguration()

	if database, err = mailslurper.ConnectToStorage(storageType, databaseConnection); err != nil {
		logger.Fatalf("Error connecting to storage type '%d' with a connection string of %s: %s", int(storageType), databaseConnection, err.Error())
		os.Exit(-1)
	}

	defer database.Disconnect()

	/*
	 * Setup the server pool
	 */
	pool := mailslurper.NewServerPool(config.MaxWorkers)

	/*
	 * Setup the SMTP listener
	 */
	smtpServer, err := mailslurper.SetupSMTPServerListener(config)
	if err != nil {
		logger.Errorf("MailSlurper: ERROR - There was a problem starting the SMTP listener: %s", err.Error())
		os.Exit(0)
	}

	defer mailslurper.CloseSMTPServerListener(smtpServer)

	/*
	 * Setup receivers (subscribers) to handle new mail items.
	 */
	receivers := []mailslurper.IMailItemReceiver{
		mailslurper.NewDatabaseReceiver(database),
	}

	/*
	 * Start the SMTP dispatcher
	 */
	go mailslurper.Dispatch(pool, smtpServer, receivers)

	/*
	 * Setup and start the HTTP listener for the application site
	 */
	setupAndStartAdminMux()

	if config.AutoStartBrowser {
		startBrowser(config)
	}

	/*
	 * Start the services server
	 */
	setupAndStartServiceTierMux()
}

func startBrowser(config *mailslurper.Configuration) {
	timer := time.NewTimer(time.Second)
	go func() {
		<-timer.C
		logger.Infof("Opening web browser to http://%s:%d\n", config.WWWAddress, config.WWWPort)
		err := open.Start(fmt.Sprintf("http://%s:%d", config.WWWAddress, config.WWWPort))
		if err != nil {
			logger.Infof("ERROR - Could not open browser - %s\n", err.Error())
		}
	}()
}

func isVerb(request *http.Request, expectedVerb string) bool {
	return strings.ToLower(request.Method) == strings.ToLower(expectedVerb)
}

func splitPath(request *http.Request) []string {
	result := strings.Split(request.URL.Path, "/")

	if len(result) > 1 {
		result = result[1:]
	}

	return result
}

func parsePath(request *http.Request, pattern string) map[string]string {
	p, _ := regexp.Compile("\\{(.*)\\}")
	splitPath := splitPath(request)
	splitPattern := strings.Split(pattern, "/")
	result := make(map[string]string)
	var key string

	if len(splitPattern) > 1 {
		splitPattern = splitPattern[1:]
	}

	for index, value := range splitPath {
		if strings.HasPrefix(splitPattern[index], "{") {
			key = p.ReplaceAllString(splitPattern[index], "$1")
		} else {
			key = splitPattern[index]
		}

		result[key] = value
	}

	return result
}

func renderMainLayout(writer http.ResponseWriter, request *http.Request, htmlFileName string, data mailslurper.Page) error {
	var layout string
	var err error
	var tmpl *template.Template
	var pageString string

	writer.Header().Set("Content-Type", "text/html; charset=UTF-8")

	/*
	 * Pre-load layout information
	 */
	if DEBUG_ASSETS {
		var bytes []byte

		if bytes, err = ioutil.ReadFile("./www/mailslurper/layouts/mainLayout.html"); err != nil {
			logger.Errorf("Error setting up layout: %s", err.Error())
			os.Exit(-1)
		}

		layout = string(bytes)
	} else {
		if layout, err = www.FSString(false, "/www/mailslurper/layouts/mainLayout.html"); err != nil {
			logger.Infof("Error setting up layout: %s", err.Error())
			os.Exit(-1)
		}
	}

	if tmpl, err = template.New("layout").Parse(layout); err != nil {
		return err
	}

	if pageString, err = getHTMLPageString(htmlFileName); err != nil {
		return err
	}

	if tmpl, err = tmpl.Parse(pageString); err != nil {
		return err
	}

	return tmpl.Execute(writer, data)
}

func getHTMLPageString(htmlFileName string) (string, error) {
	if DEBUG_ASSETS {
		bytes, err := ioutil.ReadFile("./www/" + htmlFileName)
		return string(bytes), err
	}

	return www.FSString(false, "/www/"+htmlFileName)
}
