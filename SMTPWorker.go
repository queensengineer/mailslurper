// Copyright 2013-3014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/adampresley/webframework/sanitizer"
)

/*
An SMTPWorker is responsible for executing, parsing, and processing a single
TCP connection's email.
*/
type SMTPWorker struct {
	Connection             net.Conn
	EmailValidationService EmailValidationProvider
	Mail                   MailItem
	Reader                 SMTPReader
	Receiver               chan MailItem
	State                  SMTPWorkerState
	WorkerID               int
	Writer                 SMTPWriter
	XSSService             sanitizer.IXSSServiceProvider

	pool ServerPool
}

/*
ExecuteCommand takes a command and the raw data read from the socket
connection and executes the correct handler function to process
the data and potentially respond to the client to continue SMTP negotiations.
*/
func (smtpWorker *SMTPWorker) ExecuteCommand(command SMTPCommand, streamInput string) error {
	var err error

	var headers MailHeader
	var body MailBody

	streamInput = strings.TrimSpace(streamInput)

	switch command {
	case HELO:
		err = smtpWorker.Process_HELO(streamInput)

	case MAIL:
		if err = smtpWorker.Process_MAIL(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing MAIL FROM: %s\n", err.Error())
		} else {
			log.Printf("libmailslurper: INFO - Mail from %s\n", smtpWorker.Mail.FromAddress)
		}

	case RCPT:
		if err = smtpWorker.Process_RCPT(streamInput); err != nil {
			log.Printf("libmailslurper: ERROR - Problem processing RCPT TO: %s\n", err.Error())
		}

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
func (smtpWorker *SMTPWorker) InitializeMailItem() {
	smtpWorker.Mail.ToAddresses = make([]string, 0)
	smtpWorker.Mail.Attachments = make([]*Attachment, 0)
	smtpWorker.Mail.Message = NewSMTPMessagePart()

	/*
	 * IDs are generated ahead of time because
	 * we do not know what order recievers
	 * get the mail item once it is retrieved from the TCP socket.
	 */
	id, _ := GenerateID()
	smtpWorker.Mail.ID = id
}

/*
NewSMTPWorker creates a new SMTP worker. An SMTP worker is
responsible for parsing and working with SMTP mail data.
*/
func NewSMTPWorker(
	workerID int,
	pool ServerPool,
	emailValidationService EmailValidationProvider,
	xssService sanitizer.IXSSServiceProvider,
) *SMTPWorker {
	return &SMTPWorker{
		EmailValidationService: emailValidationService,
		WorkerID:               workerID,
		State:                  SMTP_WORKER_IDLE,
		XSSService:             xssService,

		pool: pool,
	}
}

/*
ParseMailHeader, given an entire mail transmission this method parses a set of mail headers.
It will split lines up and figures out what header data goes into what
structure key. Most headers follow this format:

Header-Name: Some value here\r\n

However some headers, such as Content-Type, may have additional information,
especially when the content type is a multipart and there are attachments.
Then it can look like this:

Content-Type: multipart/mixed; boundary="==abcsdfdfd=="\r\n
*/
func (smtpWorker *SMTPWorker) ParseMailHeader(contents string) error {
	var key string

	smtpWorker.Mail.XMailer = "MailSlurper!"
	smtpWorker.Mail.Boundary = ""

	/*
	 * Split the DATA content by CRLF CRLF. The first item will be the data
	 * headers. Everything past that is body/message.
	 */
	headerBodySplit := strings.Split(contents, "\r\n\r\n")
	if len(headerBodySplit) < 2 {
		return fmt.Errorf("Expected DATA block to contain a header section and a body section")
	}

	contents = headerBodySplit[0]

	/*
	 * Unfold and split the header into lines. Loop over each line
	 * and figure out what headers are present. Store them.
	 * Sadly some headers require special processing.
	 */
	contents = UnfoldHeaders(contents)
	splitHeader := strings.Split(contents, "\r\n")
	numLines := len(splitHeader)

	for index := 0; index < numLines; index++ {
		splitItem := strings.Split(splitHeader[index], ":")
		key = splitItem[0]

		switch strings.ToLower(key) {
		case "content-type":
			contentType := strings.Join(splitItem[1:], "")
			contentTypeSplit := strings.Split(contentType, ";")

			smtpWorker.Mail.ContentType = strings.TrimSpace(contentTypeSplit[0])
			log.Println("libmailslurper: INFO - Mail Content-Type: ", smtpWorker.Mail.ContentType)

			/*
			 * Check to see if we have a boundary marker
			 */
			if len(contentTypeSplit) > 1 {
				contentTypeRightSide := strings.Join(contentTypeSplit[1:], ";")

				if strings.Contains(strings.ToLower(contentTypeRightSide), "boundary") {
					boundarySplit := strings.Split(contentTypeRightSide, "=")
					smtpWorker.Mail.Boundary = strings.Replace(strings.Join(boundarySplit[1:], "="), "\"", "", -1)

					log.Println("libmailslurper: INFO - Mail Boundary: ", smtpWorker.Mail.Boundary)
				}
			}

		case "date":
			smtpWorker.Mail.DateSent = ParseDateTime(strings.Join(splitItem[1:], ":"))
			log.Println("libmailslurper: INFO - Mail Date: ", smtpWorker.Mail.DateSent)

		case "mime-version":
			smtpWorker.Mail.MIMEVersion = strings.TrimSpace(strings.Join(splitItem[1:], ""))
			log.Println("libmailslurper: INFO - Mail MIME-Version: ", smtpWorker.Mail.MIMEVersion)

		case "subject":
			smtpWorker.Mail.Subject = strings.TrimSpace(strings.Join(splitItem[1:], ""))
			if smtpWorker.Mail.Subject == "" {
				smtpWorker.Mail.Subject = "(No Subject)"
			}

			log.Println("libmailslurper: INFO - Mail Subject: ", smtpWorker.Mail.Subject)
		}
	}

	return nil
}

/*
Prepare tells a worker about the TCP connection they will work
with, the IO handlers, and sets their state.
*/
func (smtpWorker *SMTPWorker) Prepare(
	connection net.Conn,
	receiver chan MailItem,
	reader SMTPReader,
	writer SMTPWriter,
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
func (smtpWorker *SMTPWorker) Process_DATA(streamInput string) (MailHeader, MailBody, error) {
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

func (smtpWorker *SMTPWorker) recordMessagePart(message ISMTPMessagePart) error {
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

func (smtpWorker *SMTPWorker) isMIMEType(messagePart ISMTPMessagePart, mimeType string) bool {
	return strings.HasPrefix(messagePart.GetContentType(), mimeType)
}

func (smtpWorker *SMTPWorker) messagePartIsAttachment(messagePart ISMTPMessagePart) bool {
	return strings.Contains(messagePart.GetContentDisposition(), "attachment")
}

func (smtpWorker *SMTPWorker) addAttachment(messagePart ISMTPMessagePart) error {
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
func (smtpWorker *SMTPWorker) Process_HELO(streamInput string) error {
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
func (smtpWorker *SMTPWorker) Process_MAIL(streamInput string) error {
	var err error

	if err = smtpWorker.ProcessFrom(streamInput); err != nil {
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
func (smtpWorker *SMTPWorker) Process_RCPT(streamInput string) error {
	var err error

	if err = smtpWorker.ProcessRecipient(streamInput); err != nil {
		return err
	}

	smtpWorker.Writer.SendOkResponse()
	return nil
}

/*
ProcessFrom takes the input stream and stores the sender email address. If there
is an error it is returned.
*/
func (smtpWorker *SMTPWorker) ProcessFrom(streamInput string) error {
	var err error
	var from string
	var fromComponents *mail.Address

	if err = IsValidCommand(streamInput, "MAIL FROM"); err != nil {
		return err
	}

	if from, err = GetCommandValue(streamInput, "MAIL FROM", ":"); err != nil {
		return err
	}

	if fromComponents, err = smtpWorker.EmailValidationService.GetEmailComponents(from); err != nil {
		return InvalidEmail(from)
	}

	from = smtpWorker.XSSService.SanitizeString(fromComponents.Address)

	if !smtpWorker.EmailValidationService.IsValidEmail(from) {
		return InvalidEmail(from)
	}

	smtpWorker.Mail.FromAddress = from
	return nil
}

/*
ProcessRecipient takes the input stream and stores the intended recipient(s). If there
is an error it is returned.
*/
func (smtpWorker *SMTPWorker) ProcessRecipient(streamInput string) error {
	var err error
	var to string
	var toComponents *mail.Address

	if err = IsValidCommand(streamInput, "RCPT TO"); err != nil {
		return err
	}

	if to, err = GetCommandValue(streamInput, "RCPT TO", ":"); err != nil {
		return err
	}

	if toComponents, err = smtpWorker.EmailValidationService.GetEmailComponents(to); err != nil {
		return InvalidEmail(to)
	}

	to = smtpWorker.XSSService.SanitizeString(toComponents.Address)

	if !smtpWorker.EmailValidationService.IsValidEmail(to) {
		return InvalidEmail(to)
	}

	smtpWorker.Mail.ToAddresses = append(smtpWorker.Mail.ToAddresses, to)
	return nil
}

func (smtpWorker *SMTPWorker) rejoinWorkerQueue() {
	smtpWorker.pool.JoinQueue(smtpWorker)
}

/*
Work is the function called by the SmtpListener when a client request
is received. This will start the process by responding to the client,
start processing commands, and finally close the connection.
*/
func (smtpWorker *SMTPWorker) Work() {
	go func() {
		var streamInput string
		var command SMTPCommand
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
func (smtpWorker *SMTPWorker) TimeoutHasExpired(startTime time.Time) bool {
	return int(time.Since(startTime).Seconds()) > COMMAND_TIMEOUT_SECONDS
}
