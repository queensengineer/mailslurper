package mailslurper

import (
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"

	"github.com/pkg/errors"
)

/*
An SMTPMessagePart represents a single message/content from a DATA transmission
from an SMTP client. This contains the headers and body content. It also contains
a reference to a collection of sub-messages, if any. This allows us to support
the recursive tree-like nature of the MIME protocol.
*/
type SMTPMessagePart struct {
	Message      *Message
	MessageParts []ISMTPMessagePart
}

/*
NewSMTPMessagePart returns a new instance of this struct
*/
func NewSMTPMessagePart() *SMTPMessagePart {
	return &SMTPMessagePart{
		Message:      &Message{},
		MessageParts: make([]ISMTPMessagePart, 0),
	}
}

/*
AddBody adds body content
*/
func (messagePart *SMTPMessagePart) AddBody(body string) error {
	messagePart.Message.Body = strings.NewReader(body)
	return nil
}

/*
AddHeaders takes a header set and adds it to this message part.
*/
func (messagePart *SMTPMessagePart) AddHeaders(headerSet ISet) error {
	messagePart.Message.Header = headerSet.ToMap()
	return nil
}

func (messagePart *SMTPMessagePart) BuildMessages(body string) error {
	var err error
	var headerSet ISet
	var isMultipart bool
	var boundary string

	headerBodySplit := strings.Split(body, "\r\n\r\n")
	if headerSet, err = header.NewHeaderSet(headerBodySplit[0]); err != nil {
		return errors.Wrapf(err, "Error while building message part")
	}

	if err = messagePart.AddHeaders(headerSet); err != nil {
		return errors.Wrapf(err, "Error adding headers to message part")
	}

	log.Printf("BuildMessages: adding headers %v\n", messagePart.Message.Header)

	/*
	 * If this is not a multipart message, bail early. We've got
	 * what we need.
	 */
	if isMultipart, err = messagePart.ContentIsMultipart(); err != nil {
		return errors.Wrapf(err, "Error getting content type information in message part")
	}

	if !isMultipart {
		if err = messagePart.AddBody(body); err != nil {
			return errors.Wrapf(err, "Error adding body to message part")
		}

		return nil
	}

	if boundary, err = messagePart.GetBoundary(); err != nil {
		return errors.Wrapf(err, "Error getting boundary for message part")
	}

	log.Printf("BuildMessages: boundary is %s\n", boundary)

	if err = messagePart.AddBody(body); err != nil {
		return errors.Wrapf(err, "Error adding body to message part")
	}

	log.Printf("BuildMessages: body is %s\n\n", body)

	return messagePart.ParseMessages(body, boundary)
}

func (messagePart *SMTPMessagePart) ParseMessages(body string, boundary string) error {
	var err error
	var bodyPart []byte
	var part *multipart.Part

	reader := multipart.NewReader(strings.NewReader(body), boundary)

	for {
		part, err = reader.NextPart()

		switch err {
		case io.EOF:
			log.Printf("BuildMessages: reach EOF for part\n%v\n", part)
			return nil

		case nil:
			if bodyPart, err = ioutil.ReadAll(part); err != nil {
				return errors.Wrapf(err, "Error reading body for content type '%s'", messagePart.Message.Header.Get("Content-Type"))
			}

			log.Printf("BuildMessages: building new message part:\n%s\n\n", string(bodyPart))
			if boundary, err = messagePart.GetBoundaryFromHeaderString(part.Header.Get("Content-Type")); err != nil {
				return errors.Wrapf(err, "Error getting boundary marker")
			}

			log.Printf("New boundary: %s\n", boundary)
			innerBody := string(bodyPart)

			newMessage := NewSMTPMessagePart()
			newMessage.Message.Header = messagePart.convertPartHeadersToMap(part.Header)
			newMessage.Message.Body = strings.NewReader(innerBody)

			newMessage.ParseMessages(innerBody, boundary)
			messagePart.MessageParts = append(messagePart.MessageParts, newMessage)

		default:
			return errors.Wrapf(err, "Error reading next part for content type '%s'", messagePart.Message.Header.Get("Content-Type"))
		}
	}
}

func (messagePart *SMTPMessagePart) ContentIsMultipart() (bool, error) {
	mediaType, _, err := messagePart.parseContentType()
	return strings.HasPrefix(mediaType, "multipart/"), err
}

func (messagePart *SMTPMessagePart) GetBoundary() (string, error) {
	_, boundary, err := messagePart.parseContentType()
	return boundary, err
}

func (messagePart *SMTPMessagePart) GetBoundaryFromHeaderString(header string) (string, error) {
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return "", err
	}

	return params["boundary"], nil
}

func (messagePart *SMTPMessagePart) parseContentType() (string, string, error) {
	mediaType, params, err := mime.ParseMediaType(messagePart.Message.Header.Get("Content-Type"))
	if err != nil {
		return "", "", err
	}

	return mediaType, params["boundary"], nil
}

func (messagePart *SMTPMessagePart) convertPartHeadersToMap(partHeaders textproto.MIMEHeader) map[string][]string {
	convertedHeaders := make(map[string][]string)
	for key, value := range partHeaders {
		convertedHeaders[key] = value
	}

	return convertedHeaders
}