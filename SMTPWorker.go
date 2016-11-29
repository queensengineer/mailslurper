// Copyright 2013-3014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/adampresley/webframework/sanitizer"
)

type SmtpWorker struct {
	Connection             net.Conn
	EmailValidationService EmailValidationProvider
	Mail                   MailItem
	Reader                 SmtpReader
	Receiver               chan MailItem
	SMTPMailItem           ISMTPMailItem
	State                  SmtpWorkerState
	WorkerId               int
	Writer                 SmtpWriter
	XSSService             sanitizer.IXSSServiceProvider

	pool ServerPool
}

/*
ExecuteCommand takes a command and the raw data read from the socket
connection and executes the correct handler function to process
the data and potentially respond to the client to continue SMTP negotiations.
*/
func (this *SmtpWorker) ExecuteCommand(command SmtpCommand, streamInput string) error {
	var err error

	var headers MailHeader
	var body MailBody

	streamInput = strings.TrimSpace(streamInput)
	thisMailItem := this.SMTPMailItem.(*SMTPMailItem)

	switch command {
	case HELO:
		err = this.Process_HELO(streamInput)

	case MAIL:
		if err = this.Process_MAIL(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing MAIL FROM: %s\n", err.Error())
		} else {
			log.Printf("libmailslurper: INFO - Mail from %s\n", thisMailItem.FromAddress)
		}

	case RCPT:
		if err = this.Process_RCPT(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing RCPT TO: %s\n", err.Error())
		}

	case DATA:
		headers, body, err = this.Process_DATA(streamInput)
		if err != nil {
			log.Println("libmailslurper: ERROR - Problem calling Process_DATA -", err)
		} else {
			if len(strings.TrimSpace(body.HTMLBody)) <= 0 {
				this.Mail.Body = this.XSSService.SanitizeString(body.TextBody)
			} else {
				this.Mail.Body = this.XSSService.SanitizeString(body.HTMLBody)
			}

			this.Mail.Subject = this.XSSService.SanitizeString(headers.Subject)
			this.Mail.DateSent = headers.Date
			this.Mail.XMailer = this.XSSService.SanitizeString(headers.XMailer)
			this.Mail.ContentType = this.XSSService.SanitizeString(headers.ContentType)
			this.Mail.Boundary = headers.Boundary
			this.Mail.Attachments = body.Attachments
		}

	default:
		err = nil
	}

	return err
}

/*
Initializes the mail item structure that will eventually
be written to the receiver channel.
*/
func (this *SmtpWorker) InitializeMailItem() {
	this.Mail.ToAddresses = make([]string, 0)
	this.Mail.Attachments = make([]*Attachment, 0)

	/*
	 * IDs are generated ahead of time because
	 * we do not know what order recievers
	 * get the mail item once it is retrieved from the TCP socket.
	 */
	id, _ := GenerateId()
	this.Mail.Id = id
}

/*
NewSmtpWorker creates a new SMTP worker. An SMTP worker is
responsible for parsing and working with SMTP mail data.
*/
func NewSmtpWorker(
	workerID int,
	pool ServerPool,
	emailValidationService EmailValidationProvider,
	xssService sanitizer.IXSSServiceProvider,
) *SmtpWorker {
	return &SmtpWorker{
		EmailValidationService: emailValidationService,
		WorkerId:               workerID,
		SMTPMailItem:           NewSMTPMailItem(emailValidationService, xssService),
		State:                  SMTP_WORKER_IDLE,
		XSSService:             xssService,

		pool: pool,
	}
}

/*
Prepare tells a worker about the TCP connection they will work
with, the IO handlers, and sets their state.
*/
func (this *SmtpWorker) Prepare(
	connection net.Conn,
	receiver chan MailItem,
	reader SmtpReader,
	writer SmtpWriter,
) {
	this.State = SMTP_WORKER_WORKING

	this.Connection = connection
	this.Receiver = receiver

	this.Reader = reader
	this.Writer = writer
}

/*
Function to process the DATA command (constant DATA). When a client sends the DATA
command there are three parts to the transmission content. Before this data
can be processed this function will tell the client how to terminate the DATA block.
We are asking clients to terminate with "\r\n.\r\n".

The first part is a set of header lines. Each header line is a header key (name), followed
by a colon, followed by the value for that header key. For example a header key might
be "Subject" with a value of "Testing Mail!".

After the header section there should be two sets of carriage return/line feed characters.
This signals the end of the header block and the start of the message body.

Finally when the client sends the "\r\n.\r\n" the DATA transmission portion is complete.
This function will return the following items.

	1. Headers (MailHeader)
	2. Body breakdown (MailBody)
	3. error structure
*/
func (this *SmtpWorker) Process_DATA(streamInput string) (MailHeader, MailBody, error) {
	var err error

	header := MailHeader{}
	body := MailBody{}

	commandCheck := strings.Index(strings.ToLower(streamInput), "data")
	if commandCheck < 0 {
		return header, body, errors.New("Invalid command for DATA")
	}

	this.Writer.SendDataResponse()
	entireMailContents := this.Reader.ReadDataBlock()

	/*
	 * Parse the header content
	 */
	if err = header.Parse(entireMailContents); err != nil {
		this.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
		return header, body, err
	}

	/*
	 * Parse the body. Send the
	 */
	if err = body.Parse(entireMailContents, header.Boundary); err != nil {
		this.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
		return header, body, err
	}

	this.Writer.SendOkResponse()
	return header, body, nil
}

/*
Function to process the HELO and EHLO SMTP commands. This command
responds to clients with a 250 greeting code and returns success
or false and an error message (if any).
*/
func (this *SmtpWorker) Process_HELO(streamInput string) error {
	lowercaseStreamInput := strings.ToLower(streamInput)

	commandCheck := (strings.Index(lowercaseStreamInput, "helo") + 1) + (strings.Index(lowercaseStreamInput, "ehlo") + 1)
	if commandCheck < 0 {
		return errors.New("Invalid HELO command")
	}

	split := strings.Split(streamInput, " ")
	if len(split) < 2 {
		return errors.New("HELO command format is invalid")
	}

	return this.Writer.SendHELOResponse()
}

/*
Process_MAIL processes the MAIL FROM command (constant MAIL). This command
will respond to clients with 250 Ok response and returns an error
that may have occurred as well as the parsed FROM.
*/
func (this *SmtpWorker) Process_MAIL(streamInput string) error {
	var err error

	if err = this.SMTPMailItem.ProcessFrom(streamInput); err != nil {
		return err
	}

	this.Writer.SendOkResponse()
	return nil
}

/*
Process_RCPT processes the RCPT TO command (constant RCPT). This command
will respond to clients with a 250 Ok response and returns an error structre
and a string containing the recipients address. Note that a client
can send one or more RCPT TO commands.
*/
func (this *SmtpWorker) Process_RCPT(streamInput string) error {
	var err error

	if err = this.SMTPMailItem.ProcessRecipient(streamInput); err != nil {
		return err
	}

	this.Writer.SendOkResponse()
	return nil
}

func (this *SmtpWorker) rejoinWorkerQueue() {
	this.pool.JoinQueue(this)
}

/*
This is the function called by the SmtpListener when a client request
is received. This will start the process by responding to the client,
start processing commands, and finally close the connection.
*/
func (this *SmtpWorker) Work() {
	go func() {
		var streamInput string
		var command SmtpCommand
		var err error

		this.InitializeMailItem()
		this.Writer.SayHello()

		/*
		 * Read from the connection until we receive a QUIT command
		 * or some critical error occurs and we force quit.
		 */
		startTime := time.Now()
		log.Printf("startTime: %v", startTime)

		for this.State != SMTP_WORKER_DONE && this.State != SMTP_WORKER_ERROR {
			streamInput = this.Reader.Read()
			command, err = GetCommandFromString(streamInput)

			log.Printf("command: %v", command)

			if err != nil {
				log.Println("libmailslurper: ERROR finding command from input", streamInput, "-", err)
				this.State = SMTP_WORKER_ERROR
				continue
			}

			if command == QUIT {
				this.State = SMTP_WORKER_DONE
				log.Println("libmailslurper: INFO - Closing connection")
			} else {
				err = this.ExecuteCommand(command, streamInput)
				if err != nil {
					this.State = SMTP_WORKER_ERROR
					log.Println("libmailslurper: ERROR - Error executing command", command.String())
					continue
				}
			}

			if this.TimeoutHasExpired(startTime) {
				log.Println("libmailslurper: INFO - Connection timeout. Terminating client connection")
				this.State = SMTP_WORKER_ERROR
				continue
			}
		}

		this.Writer.SayGoodbye()
		this.Connection.Close()

		if this.State != SMTP_WORKER_ERROR {
			this.Receiver <- this.Mail
		}

		this.State = SMTP_WORKER_IDLE
		this.rejoinWorkerQueue()
	}()
}

/*
Determines if the time elapsed since a start time has exceeded
the command timeout.
*/
func (this *SmtpWorker) TimeoutHasExpired(startTime time.Time) bool {
	return int(time.Since(startTime).Seconds()) > COMMAND_TIMEOUT_SECONDS
}
