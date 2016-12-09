// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/adampresley/webframework/logging2"
)

/*
SetupSMTPServerListener establishes a listening connection to a socket on an address. This will
return a net.Listener handle.
*/
func SetupSMTPServerListener(config *Configuration, logger logging2.ILogger) (net.Listener, error) {
	var tcpAddress *net.TCPAddr
	var certificate tls.Certificate
	var err error

	if config.CertFile != "" && config.KeyFile != "" {
		if certificate, err = tls.LoadX509KeyPair(config.CertFile, config.KeyFile); err != nil {
			return &net.TCPListener{}, err
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{certificate}}

		logger.Infof("SMTP listener running on SSL - %s", config.GetFullSMTPBindingAddress())
		return tls.Listen("tcp", config.GetFullSMTPBindingAddress(), tlsConfig)
	}

	if tcpAddress, err = net.ResolveTCPAddr("tcp", config.GetFullSMTPBindingAddress()); err != nil {
		return &net.TCPListener{}, err
	}

	logger.Infof("SMTP listener running on %s", config.GetFullSMTPBindingAddress())
	return net.ListenTCP("tcp", tcpAddress)
}

/*
CloseSMTPServerListener closes a socket connection in an Server object. Most likely used in a defer call.
*/
func CloseSMTPServerListener(handle net.Listener) error {
	return handle.Close()
}

/*
Dispatch starts the process of handling SMTP client connections.
The first order of business is to setup a channel for writing
parsed mails, in the form of MailItemStruct variables, to our
database. A goroutine is setup to listen on that
channel and handles storage.

Meanwhile this method will loop forever and wait for client connections (blocking).
When a connection is recieved a goroutine is started to create a new MailItemStruct
and parser and the parser process is started. If the parsing is successful
the MailItemStruct is added to a channel. An receivers passed in will be
listening on that channel and may do with the mail item as they wish.
*/
func Dispatch(serverPool ServerPool, handle net.Listener, receivers []IMailItemReceiver, logger logging2.ILogger, killChannel chan bool, wg *sync.WaitGroup) {
	/*
	 * Setup our receivers. These guys are basically subscribers to
	 * the MailItem channel.
	 */
	mailItemChannel := make(chan MailItem, 1000)
	killReceiverChannel := make(chan bool, 1)

	var worker *SMTPWorker

	wg.Add(2)

	go func() {
		logger.Infof("%d receiver(s) listening", len(receivers))

		for {
			select {
			case item := <-mailItemChannel:
				for _, r := range receivers {
					go r.Receive(&item, wg)
				}

			case <-killReceiverChannel:
				logger.Debugf("Shutting down receiver channel...")
				wg.Done()
				break
			}
		}
	}()

	/*
	 * Now start accepting connections for SMTP
	 */
	go func() {
		for {
			select {
			case <-killChannel:
				break

			default:
				connection, err := handle.Accept()
				if err != nil {
					logger.Errorf("Problem accepting SMTP requests - %s", err.Error())
					killChannel <- true
					break
				}

				if worker, err = serverPool.NextWorker(connection, mailItemChannel); err != nil {
					connection.Close()
					logger.Errorf(err.Error())
					continue
				}

				go worker.Work()
			}
		}
	}()

	<-killChannel
	logger.Debugf("Received kill code..")
	wg.Done()

	killReceiverChannel <- true
}
