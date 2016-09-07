// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

//go:generate esc -o ./www/www.go -pkg www -ignore DS_Store|README\.md|LICENSE -prefix /www/ ./www

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/adampresley/webframework/console"
	"github.com/adampresley/webframework/logging"
	"github.com/mailslurper/libmailslurper/configuration"
	"github.com/mailslurper/libmailslurper/server"
	"github.com/mailslurper/mailslurper"
	"github.com/mailslurper/mailslurper/cmd/mailslurper/global"
	"github.com/skratchdot/open-golang/open"
)

const (
	// Version of the MailSlurper Server application
	SERVER_VERSION string = "1.11.1"

	// Set to true while developing
	DEBUG_ASSETS bool = false

	CONFIGURATION_FILE_NAME string = "config.json"
)

var config *mailslurper.Configuration
var database mailslurper.IStorage
var log *logging.Logger

func main() {
	var err error

	log = logging.NewLoggerWithMinimumLevel("MailSlurper", logging.StringToLogType("debug"))
	log.Infof("Starting MailSlurper Server v%s", SERVER_VERSION)

	/*
	 * Prepare SIGINT handler (CTRL+C)
	 */
	console.ListenForSIGINT(func() {
		log.Infof("Shutting down")
		os.Exit(0)
	})

	/*
	 * Load configuration
	 */
	if config, err = mailslurper.LoadConfigurationFromFile(CONFIGURATION_FILE_NAME); err != nil {
		log.Fatalf("There was an error reading the configuration file '%s': %s", CONFIGURATION_FILE_NAME, err.Error())
		os.Exit(-1)
	}

	/*
	 * Setup global database connection handle
	 */
	storageType, databaseConnection := config.GetDatabaseConfiguration()

	if database, err = mailslurper.ConnectToStorage(storageType, databaseConnection); err != nil {
		log.Fatalf("Error connecting to storage type '%d' with a connection string of %s: %s", int(storageType), databaseConnection, err.Error())
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
		log.Println("MailSlurper: ERROR - There was a problem starting the SMTP listener:", err)
		os.Exit(0)
	}

	defer CloseSMTPServerListener(smtpServer)

	/*
	 * Setup receivers (subscribers) to handle new mail items.
	 */
	receivers := []mailslurper.IMailItemReceiver{
		mailslurper.NewDatabaseReceiver(database),
	}

	/*
	 * Start the SMTP dispatcher
	 */
	go server.Dispatch(pool, smtpServer, receivers)

	/*
	 * Setup and start the HTTP listener for the application site
	 */
	adminMux := http.NewServeMux()

	adminMux.Handle("/www/", http.StripPrefix("/www/", http.FileServer(http.Dir("./www/"))))
	adminMux.Handle("/", baseMiddleware(http.HandlerFunc(index)))
	adminMux.Handle("/admin", baseMiddleware(http.HandlerFunc(admin)))
	adminMux.Handle("/savedsearches", baseMiddleware(http.HandlerFunc(manageSavedSearches)))
	adminMux.Handle("/servicesettings", baseMiddleware(http.HandlerFunc(getServiceSettings)))
	adminMux.Handle("/version", baseMiddleware(http.HandlerFunc(getVersion)))

	go func() {
		if err := http.ListenAndServe(config.GetFullWWWBindingAddress(), adminMux); err != nil {
			log.Fatalf("Error starting HTTP admin listener: %s", err.Error())
			os.Exit(-1)
		}
	}()

	if config.AutoStartBrowser {
		startBrowser(config)
	}

	/*
	 * Start the services server
	 */
	serviceTierConfiguration := &mailslurper.ServiceTierConfiguration{
		Address:  config.ServiceAddress,
		Port:     config.ServicePort,
		Database: global.Database,
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
	}

	serviceMux := http.NewServeMux()

	serviceMux.Handle("/version", baseMiddleware(http.HandlerFunc(version)))
	serviceMux.Handle("/mail", baseMiddleware(http.HandlerFunc(mailEndpoint)))

	if err = mailslurper.StartServiceTier(serviceTierConfiguration); err != nil {
		log.Printf("MailSlurper: ERROR - Error starting MailSlurper services server: %s\n", err.Error())
		os.Exit(1)
	}
}

func startBrowser(config *configuration.Configuration) {
	timer := time.NewTimer(time.Second)
	go func() {
		<-timer.C
		log.Printf("Opening web browser to http://%s:%d\n", config.WWWAddress, config.WWWPort)
		err := open.Start(fmt.Sprintf("http://%s:%d", config.WWWAddress, config.WWWPort))
		if err != nil {
			log.Printf("ERROR - Could not open browser - %s\n", err.Error())
		}
	}()
}
