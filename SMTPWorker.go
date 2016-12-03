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

/*
An SmtpWorker is responsible for executing, parsing, and processing a single
TCP connection's email.
*/
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
func (smtpWorker *SmtpWorker) ExecuteCommand(command SmtpCommand, streamInput string) error {
	var err error

	var headers MailHeader
	var body MailBody

	streamInput = strings.TrimSpace(streamInput)
	thisMailItem := smtpWorker.SMTPMailItem.(*SMTPMailItem)

	switch command {
	case HELO:
		err = smtpWorker.Process_HELO(streamInput)

	case MAIL:
		if err = smtpWorker.Process_MAIL(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing MAIL FROM: %s\n", err.Error())
		} else {
			log.Printf("libmailslurper: INFO - Mail from %s\n", thisMailItem.FromAddress)
		}

		smtpWorker.Mail.FromAddress = thisMailItem.FromAddress

	case RCPT:
		if err = smtpWorker.Process_RCPT(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing RCPT TO: %s\n", err.Error())
		}

		smtpWorker.Mail.ToAddresses = thisMailItem.ToAddresses

	case DATA:
		headers, body, err = smtpWorker.Process_DATA(streamInput)
		if err != nil {
			log.Println("libmailslurper: ERROR - Problem calling Process_DATA -", err)
		} else {
			if len(strings.TrimSpace(body.HTMLBody)) <= 0 {
				smtpWorker.Mail.Body = smtpWorker.XSSService.SanitizeString(body.TextBody)
			} else {
				smtpWorker.Mail.Body = smtpWorker.XSSService.SanitizeString(body.HTMLBody)
			}

			smtpWorker.Mail.Subject = smtpWorker.XSSService.SanitizeString(headers.Subject)
			smtpWorker.Mail.DateSent = headers.Date
			smtpWorker.Mail.XMailer = smtpWorker.XSSService.SanitizeString(headers.XMailer)
			smtpWorker.Mail.ContentType = smtpWorker.XSSService.SanitizeString(headers.ContentType)
			smtpWorker.Mail.Boundary = headers.Boundary
			smtpWorker.Mail.Attachments = body.Attachments
		}

	default:
		err = nil
	}

	return err
}

/*
InitializeMailItem initializes the mail item structure that will eventually
be written to the receiver channel.
*/
func (smtpWorker *SmtpWorker) InitializeMailItem() {
	smtpWorker.Mail.ToAddresses = make([]string, 0)
	smtpWorker.Mail.Attachments = make([]*Attachment, 0)
	smtpWorker.Mail.Message = NewSMTPMessagePart()

	/*
	 * IDs are generated ahead of time because
	 * we do not know what order recievers
	 * get the mail item once it is retrieved from the TCP socket.
	 */
	id, _ := GenerateId()
	smtpWorker.Mail.ID = id
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
func (smtpWorker *SmtpWorker) Prepare(
	connection net.Conn,
	receiver chan MailItem,
	reader SmtpReader,
	writer SmtpWriter,
) {
	smtpWorker.State = SMTP_WORKER_WORKING

	smtpWorker.Connection = connection
	smtpWorker.Receiver = receiver

	smtpWorker.Reader = reader
	smtpWorker.Writer = writer
}

/*
Process_DATA processes the DATA command (constant DATA). When a client sends the DATA
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
func (smtpWorker *SmtpWorker) Process_DATA(streamInput string) (MailHeader, MailBody, error) {
	var err error

	header := MailHeader{}
	body := MailBody{}

	commandCheck := strings.Index(strings.ToLower(streamInput), "data")
	if commandCheck < 0 {
		return header, body, errors.New("Invalid command for DATA")
	}

	smtpWorker.Writer.SendDataResponse()
	entireMailContents := smtpWorker.Reader.ReadDataBlock()

	/*
	 * Parse the header content
	 */
	if err = header.Parse(entireMailContents); err != nil {
		smtpWorker.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
		return header, body, err
	}

	/*
		 * Parse the body. Send the
		if err = body.Parse(entireMailContents, header.Boundary); err != nil {
			smtpWorker.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
			return header, body, err
		}
	*/

	if err = smtpWorker.Mail.Message.BuildMessages(entireMailContents); err != nil {
		log.Printf("libmailslurper: ERROR parsing message contents: %s", err.Error())
	}

	/*
		t, _ := json.MarshalIndent(smtpWorker.Mail.Message, "", "  ")
		log.Printf("libmailslurper: INFO - Message Parts: %s", string(t))

		log.Printf("Message: %s", smtpWorker.Mail.Message.MessageParts[0].GetMessageParts()[1].GetBody())
	*/

	if len(smtpWorker.Mail.Message.MessageParts) > 0 {
		smtpWorker.recordMessagePart(smtpWorker.Mail.Message.MessageParts[0])
	} else {
		log.Printf("libmailslurper: ERROR - MessageParts has no parts!")
	}

	body.HTMLBody = smtpWorker.Mail.HTMLBody
	body.TextBody = smtpWorker.Mail.TextBody

	if smtpWorker.Mail.HTMLBody != "" {
		smtpWorker.Mail.Body = smtpWorker.Mail.HTMLBody
	} else {
		smtpWorker.Mail.Body = smtpWorker.Mail.TextBody
	}

	smtpWorker.Writer.SendOkResponse()
	return header, body, nil
}

func (smtpWorker *SmtpWorker) recordMessagePart(message ISMTPMessagePart) error {
	if smtpWorker.isMIMEType(message, "text/plain") && smtpWorker.Mail.TextBody == "" && !smtpWorker.messagePartIsAttachment(message) {
		smtpWorker.Mail.TextBody = message.GetBody()
	} else {
		if smtpWorker.isMIMEType(message, "text/html") && smtpWorker.Mail.HTMLBody == "" && !smtpWorker.messagePartIsAttachment(message) {
			smtpWorker.Mail.HTMLBody = message.GetBody()
		} else {
			if smtpWorker.isMIMEType(message, "multipart") {
				for _, m := range message.GetMessageParts() {
					smtpWorker.recordMessagePart(m)
				}
			} else {
				smtpWorker.addAttachment(message)
			}
		}
	}

	return nil
}

func (smtpWorker *SmtpWorker) isMIMEType(messagePart ISMTPMessagePart, mimeType string) bool {
	return strings.HasPrefix(messagePart.GetContentType(), mimeType)
}

func (smtpWorker *SmtpWorker) messagePartIsAttachment(messagePart ISMTPMessagePart) bool {
	return strings.Contains(messagePart.GetContentDisposition(), "attachment")
}

func (smtpWorker *SmtpWorker) addAttachment(messagePart ISMTPMessagePart) error {
	headers := &AttachmentHeader{
		ContentType:             messagePart.GetHeader("Content-Type"),
		MIMEVersion:             messagePart.GetHeader("MIME-Version"),
		ContentTransferEncoding: messagePart.GetHeader("Content-Transfer-Encoding"),
		ContentDisposition:      messagePart.GetContentDisposition(),
		FileName:                messagePart.GetFilenameFromContentDisposition(),
	}

	log.Printf("libmailslurper: INFO - Attachment: %v", headers)

	attachment := NewAttachment(headers, messagePart.GetBody())

	if smtpWorker.messagePartIsAttachment(messagePart) {
		smtpWorker.Mail.Attachments = append(smtpWorker.Mail.Attachments, attachment)
	} else {
		smtpWorker.Mail.InlineAttachments = append(smtpWorker.Mail.InlineAttachments, attachment)
	}

	return nil
}

/*
Process_HELO processes the HELO and EHLO SMTP commands. This command
responds to clients with a 250 greeting code and returns success
or false and an error message (if any).
*/
func (smtpWorker *SmtpWorker) Process_HELO(streamInput string) error {
	lowercaseStreamInput := strings.ToLower(streamInput)

	commandCheck := (strings.Index(lowercaseStreamInput, "helo") + 1) + (strings.Index(lowercaseStreamInput, "ehlo") + 1)
	if commandCheck < 0 {
		return errors.New("Invalid HELO command")
	}

	split := strings.Split(streamInput, " ")
	if len(split) < 2 {
		return errors.New("HELO command format is invalid")
	}

	return smtpWorker.Writer.SendHELOResponse()
}

/*
Process_MAIL processes the MAIL FROM command (constant MAIL). This command
will respond to clients with 250 Ok response and returns an error
that may have occurred as well as the parsed FROM.
*/
func (smtpWorker *SmtpWorker) Process_MAIL(streamInput string) error {
	var err error

	if err = smtpWorker.SMTPMailItem.ProcessFrom(streamInput); err != nil {
		return err
	}

	smtpWorker.Writer.SendOkResponse()
	return nil
}

/*
Process_RCPT processes the RCPT TO command (constant RCPT). This command
will respond to clients with a 250 Ok response and returns an error structre
and a string containing the recipients address. Note that a client
can send one or more RCPT TO commands.
*/
func (smtpWorker *SmtpWorker) Process_RCPT(streamInput string) error {
	var err error

	if err = smtpWorker.SMTPMailItem.ProcessRecipient(streamInput); err != nil {
		return err
	}

	smtpWorker.Writer.SendOkResponse()
	return nil
}

func (smtpWorker *SmtpWorker) rejoinWorkerQueue() {
	smtpWorker.pool.JoinQueue(smtpWorker)
}

/*
Work is the function called by the SmtpListener when a client request
is received. This will start the process by responding to the client,
start processing commands, and finally close the connection.
*/
func (smtpWorker *SmtpWorker) Work() {
	go func() {
		var streamInput string
		var command SmtpCommand
		var err error

		smtpWorker.InitializeMailItem()
		smtpWorker.Writer.SayHello()

		/*
		 * Read from the connection until we receive a QUIT command
		 * or some critical error occurs and we force quit.
		 */
		startTime := time.Now()

		for smtpWorker.State != SMTP_WORKER_DONE && smtpWorker.State != SMTP_WORKER_ERROR {
			streamInput = smtpWorker.Reader.Read()
			command, err = GetCommandFromString(streamInput)

			if err != nil {
				log.Println("libmailslurper: ERROR finding command from input", streamInput, "-", err)
				smtpWorker.State = SMTP_WORKER_ERROR
				continue
			}

			if command == QUIT {
				smtpWorker.State = SMTP_WORKER_DONE
				log.Println("libmailslurper: INFO - Closing connection")
			} else {
				err = smtpWorker.ExecuteCommand(command, streamInput)
				if err != nil {
					smtpWorker.State = SMTP_WORKER_ERROR
					log.Println("libmailslurper: ERROR - Error executing command", command.String())
					continue
				}
			}

			if smtpWorker.TimeoutHasExpired(startTime) {
				log.Println("libmailslurper: INFO - Connection timeout. Terminating client connection")
				smtpWorker.State = SMTP_WORKER_ERROR
				continue
			}
		}

		smtpWorker.Writer.SayGoodbye()
		smtpWorker.Connection.Close()

		if smtpWorker.State != SMTP_WORKER_ERROR {
			smtpWorker.Receiver <- smtpWorker.Mail
		}

		smtpWorker.State = SMTP_WORKER_IDLE
		smtpWorker.rejoinWorkerQueue()
	}()
}

/*
TimeoutHasExpired determines if the time elapsed since a start time has exceeded
the command timeout.
*/
func (smtpWorker *SmtpWorker) TimeoutHasExpired(startTime time.Time) bool {
	return int(time.Since(startTime).Seconds()) > COMMAND_TIMEOUT_SECONDS
}
