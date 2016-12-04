// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"fmt"
	"strings"

	"github.com/adampresley/webframework/logging2"
)

/*
A MailBody is the body portion of a mail
*/
type MailBody struct {
	TextBody    string
	HTMLBody    string
	Attachments []*Attachment

	logger logging2.ILogger
}

/*
NewMailBody creates a new MailBody object
*/
func NewMailBody(textBody, htmlBody string, attachments []*Attachment, logger logging2.ILogger) *MailBody {
	return &MailBody{
		TextBody:    textBody,
		HTMLBody:    htmlBody,
		Attachments: attachments,

		logger: logger,
	}
}

/*
Parses a mail's DATA section. This will attempt to figure out
what this mail contains. At the simplest level it will contain
a text message. A more complex example would be a multipart message
with mixed text and HTML. It will also parse any attachments and
retrieve their contents into an attachments array.
*/
func (mailBody *MailBody) Parse(contents string, boundary string) error {
	mailBody.logger.Debugf("Full body of message == %s", contents)

	/*
	 * Split the DATA content by CRLF CRLF. The first item will be the data
	 * headers. Everything past that is body/message.
	 */
	headerBodySplit := strings.Split(contents, "\r\n\r\n")
	if len(headerBodySplit) < 2 {
		return fmt.Errorf("Expected DATA block to contain a header section and a body section")
	}

	contents = strings.Join(headerBodySplit[1:], "\r\n\r\n")
	mailBody.Attachments = make([]*Attachment, 0)

	/*
	 * If there is no boundary then this is the simplest
	 * plain text type of mail you can get.
	 */
	if len(boundary) <= 0 {
		mailBody.TextBody = contents
	} else {
		bodyParts := strings.Split(strings.TrimSpace(contents), fmt.Sprintf("--%s", strings.TrimSpace(boundary)))
		var index int

		/*
		 * First parse the headers for each of these attachments, then
		 * place each where they go.
		 */
		for index = 0; index < len(bodyParts); index++ {
			if len(strings.TrimSpace(bodyParts[index])) <= 0 || strings.TrimSpace(bodyParts[index]) == "--" {
				continue
			}

			header := &AttachmentHeader{}
			header.Parse(strings.TrimSpace(bodyParts[index]))

			switch {
			case strings.Contains(header.ContentType, "text/plain"):
				mailBody.TextBody = header.Body

			case strings.Contains(header.ContentType, "text/html"):
				mailBody.HTMLBody = header.Body

			case strings.Contains(header.ContentDisposition, "attachment"):
				newAttachment := &Attachment{
					Headers:  header,
					Contents: header.Body,
				}

				mailBody.Attachments = append(mailBody.Attachments, newAttachment)
			}
		}
	}

	return nil
}
