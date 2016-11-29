package mailslurper

import (
	"net/mail"

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
	XSSService             sanitizer.IXSSServiceProvider
}

/*
NewSMTPMailItem returns a new instance
*/
func NewSMTPMailItem(emailValidationService EmailValidationProvider, xssService sanitizer.IXSSServiceProvider) *SMTPMailItem {
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
	var fromComponents *mail.Address

	if err = IsValidCommand(streamInput, "MAIL FROM"); err != nil {
		return err
	}

	if from, err = GetCommandValue(streamInput, "MAIL FROM", ":"); err != nil {
		return err
	}

	if fromComponents, err = mailItem.EmailValidationService.GetEmailComponents(from); err != nil {
		return InvalidEmail(from)
	}

	from = mailItem.XSSService.SanitizeString(fromComponents.Address)

	if !mailItem.EmailValidationService.IsValidEmail(from) {
		return InvalidEmail(from)
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
	var toComponents *mail.Address

	if err = IsValidCommand(streamInput, "RCPT TO"); err != nil {
		return err
	}

	if to, err = GetCommandValue(streamInput, "RCPT TO", ":"); err != nil {
		return err
	}

	if toComponents, err = mailItem.EmailValidationService.GetEmailComponents(to); err != nil {
		return InvalidEmail(to)
	}

	to = mailItem.XSSService.SanitizeString(toComponents.Address)

	if !mailItem.EmailValidationService.IsValidEmail(to) {
		return InvalidEmail(to)
	}

	mailItem.ToAddresses = append(mailItem.ToAddresses, to)
	return nil
}
