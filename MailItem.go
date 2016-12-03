// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

/*
MailItem is a struct describing a parsed mail item. This is
populated after an incoming client connection has finished
sending mail data to this server.
*/
type MailItem struct {
	ID          string        `json:"id"`
	DateSent    string        `json:"dateSent"`
	FromAddress string        `json:"fromAddress"`
	ToAddresses []string      `json:"toAddresses"`
	Subject     string        `json:"subject"`
	XMailer     string        `json:"xmailer"`
	MIMEVersion string        `json:"mimeVersion"`
	Body        string        `json:"body"`
	ContentType string        `json:"contentType"`
	Boundary    string        `json:"boundary"`
	Attachments []*Attachment `json:"attachments"`

	Message           *SMTPMessagePart
	InlineAttachments []*Attachment
	TextBody          string
	HTMLBody          string
}

func NewMailItem(id, dateSent, fromAddress string, toAddresses []string, subject, xMailer, body, contentType, boundary string, attachments []*Attachment) *MailItem {
	return &MailItem{
		ID:          id,
		DateSent:    dateSent,
		FromAddress: fromAddress,
		ToAddresses: toAddresses,
		Subject:     subject,
		XMailer:     xMailer,
		Body:        body,
		ContentType: contentType,
		Boundary:    boundary,
		Attachments: attachments,

		Message: NewSMTPMessagePart(),
	}
}
