package mailslurper

import (
	"github.com/adampresley/webframework/sanitizer"
)

/*
An SMTPMailItem represents the raw and semi-processed data recieved from an SMTP client.
This is used while communicating with a client to track recipients, senders, and
messages (bodies, attachments).
*/
type SMTPMailItem struct {
	FromAddress string
	ToAddresses []string
	Message     ISMTPMessagePart

	EmailValidationService EmailValidationProvider
	XSSService             sanitizer.XSSServiceProvider
}

/*
NewSMTPMailItem returns a new instance
*/
func NewSMTPMailItem(emailValidationService EmailValidationProvider, xssService sanitizer.XSSServiceProvider) *SMTPMailItem {
	return &SMTPMailItem{
		ToAddresses: make([]string, 0),

		EmailValidationService: emailValidationService,
		XSSService:             xssService,
	}
}

/*
ProcessBody takes the input stream and processes body and attachments.
This will produce SMTPMessageParts. This processor attempts to be
MIME compliant and understands text/plain, text/html, multipart/related,
multipart/mixed, and multipart/alternative implementations. It will also
attempt to reconstruct HTML bodies with references to embedded items
via Content-Id.
*/
func (mailItem *SMTPMailItem) ProcessBody(streamInput string) error {
	mailItem.Message = NewSMTPMessagePart()
	return mailItem.Message.BuildMessages(streamInput)
}

/*
ProcessFrom takes the input stream and stores the sender email address. If there
is an error it is returned.
*/
func (mailItem *SMTPMailItem) ProcessFrom(streamInput string) error {
	var err error
	var from string

	if err = validation.IsValidCommand(streamInput, "MAIL FROM"); err != nil {
		return err
	}

	if from, err = smtputilities.GetCommandValue(streamInput, "MAIL FROM", ":"); err != nil {
		return err
	}

	from = mailItem.XSSService.SanitizeString(from)

	if !mailItem.EmailValidationService.IsValidEmail(from) {
		return smtperrors.InvalidEmail(from)
	}

	mailItem.FromAddress = from
	return nil
}

/*
ProcessRecipient takes the input stream and stores the intended recipient(s). If there
is an error it is returned.
*/
func (mailItem *SMTPMailItem) ProcessRecipient(streamInput string) error {
	var err error
	var to string

	if err = validation.IsValidCommand(streamInput, "RCPT TO"); err != nil {
		return err
	}

	if to, err = smtputilities.GetCommandValue(streamInput, "RCPT TO", ":"); err != nil {
		return err
	}

	to = mailItem.XSSService.SanitizeString(to)

	if !mailItem.EmailValidationService.IsValidEmail(to) {
		return smtperrors.InvalidEmail(to)
	}

	mailItem.ToAddresses = append(mailItem.ToAddresses, to)
	return nil
}
