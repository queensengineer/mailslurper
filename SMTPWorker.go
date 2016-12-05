// Copyright 2013-3014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"bufio"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/adampresley/webframework/logging2"
	"github.com/adampresley/webframework/sanitizer"
	"github.com/pkg/errors"
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

	pool   ServerPool
	logger logging2.ILogger
}

/*
ExecuteCommand takes a command and the raw data read from the socket
connection and executes the correct handler function to process
the data and potentially respond to the client to continue SMTP negotiations.
*/
func (smtpWorker *SMTPWorker) ExecuteCommand(command SMTPCommand, streamInput string) error {
	var err error
	streamInput = strings.TrimSpace(streamInput)

	switch command {
	case HELO:
		err = smtpWorker.ProcessHELO(streamInput)

	case MAIL:
		if err = smtpWorker.ProcessMAIL(streamInput); err != nil {
			smtpWorker.logger.Errorf("Problem processing MAIL FROM: %s", err.Error())
		} else {
			smtpWorker.logger.Infof("Mail from %s", smtpWorker.Mail.FromAddress)
		}

	case RCPT:
		if err = smtpWorker.ProcessRCPT(streamInput); err != nil {
			smtpWorker.logger.Errorf("Problem processing RCPT TO: %s", err.Error())
		}

	case DATA:
		if err = smtpWorker.ProcessDATA(streamInput); err != nil {
			smtpWorker.logger.Errorf("Problem calling Process_DATA: %s", err.Error())
		} else {
			smtpWorker.Mail.Body = smtpWorker.XSSService.SanitizeString(smtpWorker.Mail.Body)
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
	smtpWorker.Mail.Message = NewSMTPMessagePart(smtpWorker.logger)

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
	logger logging2.ILogger,
) *SMTPWorker {
	return &SMTPWorker{
		EmailValidationService: emailValidationService,
		WorkerID:               workerID,
		State:                  SMTP_WORKER_IDLE,
		XSSService:             xssService,

		pool:   pool,
		logger: logger,
	}
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
ProcessDATA processes the DATA command (constant DATA). When a client sends the DATA
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
func (smtpWorker *SMTPWorker) ProcessDATA(streamInput string) error {
	var err error
	var initialHeaders textproto.MIMEHeader

	commandCheck := strings.Index(strings.ToLower(streamInput), "data")
	if commandCheck < 0 {
		return errors.New("Invalid command for DATA")
	}

	smtpWorker.Writer.SendDataResponse()

	entireMailContents := smtpWorker.Reader.ReadDataBlock()
	headerReader := textproto.NewReader(bufio.NewReader(strings.NewReader(entireMailContents)))

	if initialHeaders, err = headerReader.ReadMIMEHeader(); err != nil {
		return errors.Wrapf(err, "Unable to read MIME header for data block")
	}

	/*
	 * This is a simple text-based email
	 */
	if strings.Contains(initialHeaders.Get("Content-Type"), "text/plain") {
		smtpWorker.processTextMail(initialHeaders, entireMailContents)
		smtpWorker.Writer.SendOkResponse()
		return nil
	}

	/*
	 * This is a simple HTML email
	 */
	if strings.Contains(initialHeaders.Get("Content-Type"), "text/html") {
		smtpWorker.processHTMLMail(initialHeaders, entireMailContents)
		smtpWorker.Writer.SendOkResponse()
		return nil
	}

	/*
	 * Nothing simple here. We have some type of multipart email
	 */
	if err = smtpWorker.Mail.Message.BuildMessages(entireMailContents); err != nil {
		smtpWorker.logger.Errorf("Problem parsing message contents: %s", err.Error())
		smtpWorker.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
		return errors.Wrap(err, "Problem parsing message contents")
	}

	smtpWorker.Mail.Subject = smtpWorker.Mail.Message.GetHeader("Subject")
	smtpWorker.Mail.DateSent = ParseDateTime(smtpWorker.Mail.Message.GetHeader("Date"), smtpWorker.logger)
	smtpWorker.Mail.ContentType = smtpWorker.Mail.Message.GetHeader("Content-Type")

	if len(smtpWorker.Mail.Message.MessageParts) > 0 {
		for _, m := range smtpWorker.Mail.Message.MessageParts {
			smtpWorker.recordMessagePart(m)
		}
	} else {
		smtpWorker.logger.Errorf("MessagePart has no parts!")
		smtpWorker.Writer.SendResponse(SMTP_ERROR_TRANSACTION_FAILED)
		return errors.New("Message part has no parts!")
	}

	if smtpWorker.Mail.HTMLBody != "" {
		smtpWorker.Mail.Body = smtpWorker.Mail.HTMLBody
	} else {
		smtpWorker.Mail.Body = smtpWorker.Mail.TextBody
	}

	smtpWorker.Writer.SendOkResponse()
	return nil
}

func (smtpWorker *SMTPWorker) processTextMail(headers textproto.MIMEHeader, contents string) error {
	var err error

	smtpWorker.Mail.Subject = headers.Get("Subject")
	smtpWorker.Mail.DateSent = ParseDateTime(headers.Get("Date"), smtpWorker.logger)
	smtpWorker.Mail.ContentType = headers.Get("Content-Type")
	smtpWorker.Mail.TextBody, err = smtpWorker.getBodyContent(contents)
	smtpWorker.Mail.Body = smtpWorker.Mail.TextBody

	return err
}

func (smtpWorker *SMTPWorker) processHTMLMail(headers textproto.MIMEHeader, contents string) error {
	var err error

	smtpWorker.Mail.Subject = headers.Get("Subject")
	smtpWorker.Mail.DateSent = ParseDateTime(headers.Get("Date"), smtpWorker.logger)
	smtpWorker.Mail.ContentType = headers.Get("Content-Type")
	smtpWorker.Mail.HTMLBody, err = smtpWorker.getBodyContent(contents)
	smtpWorker.Mail.Body = smtpWorker.Mail.HTMLBody

	return err
}

func (smtpWorker *SMTPWorker) getBodyContent(contents string) (string, error) {
	/*
	 * Split the DATA content by CRLF CRLF. The first item will be the data
	 * headers. Everything past that is body/message.
	 */
	headerBodySplit := strings.Split(contents, "\r\n\r\n")
	if len(headerBodySplit) < 2 {
		return "", errors.New("Expected DATA block to contain a header section and a body section")
	}

	return strings.Join(headerBodySplit[1:], "\r\n\r\n"), nil
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

	smtpWorker.logger.Debugf("Adding attachment: %v", headers)

	attachment := NewAttachment(headers, messagePart.GetBody())

	if smtpWorker.messagePartIsAttachment(messagePart) {
		smtpWorker.Mail.Attachments = append(smtpWorker.Mail.Attachments, attachment)
	} else {
		smtpWorker.Mail.InlineAttachments = append(smtpWorker.Mail.InlineAttachments, attachment)
	}

	return nil
}

/*
ProcessHELO processes the HELO and EHLO SMTP commands. This command
responds to clients with a 250 greeting code and returns success
or false and an error message (if any).
*/
func (smtpWorker *SMTPWorker) ProcessHELO(streamInput string) error {
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
ProcessMAIL processes the MAIL FROM command (constant MAIL). This command
will respond to clients with 250 Ok response and returns an error
that may have occurred as well as the parsed FROM.
*/
func (smtpWorker *SMTPWorker) ProcessMAIL(streamInput string) error {
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
	smtpWorker.Writer.SendOkResponse()
	return nil
}

/*
ProcessRCPT processes the RCPT TO command (constant RCPT). This command
will respond to clients with a 250 Ok response and returns an error structre
and a string containing the recipients address. Note that a client
can send one or more RCPT TO commands.
*/
func (smtpWorker *SMTPWorker) ProcessRCPT(streamInput string) error {
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
	smtpWorker.Writer.SendOkResponse()
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
			smtpWorker.logger.Errorf("Problem finding command from input %s: %s", streamInput, err.Error())
			smtpWorker.State = SMTP_WORKER_ERROR
			continue
		}

		if command == QUIT {
			smtpWorker.State = SMTP_WORKER_DONE
			smtpWorker.logger.Infof("Closing connection")
		} else {
			if err = smtpWorker.ExecuteCommand(command, streamInput); err != nil {
				smtpWorker.State = SMTP_WORKER_ERROR
				smtpWorker.logger.Errorf("Problem executing command %s", command.String())
				continue
			}
		}

		if smtpWorker.TimeoutHasExpired(startTime) {
			smtpWorker.logger.Infof("Connection timeout. Terminating client connection")
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
}

/*
TimeoutHasExpired determines if the time elapsed since a start time has exceeded
the command timeout.
*/
func (smtpWorker *SMTPWorker) TimeoutHasExpired(startTime time.Time) bool {
	return int(time.Since(startTime).Seconds()) > COMMAND_TIMEOUT_SECONDS
}
