package mailslurper

/*
An ISMTPMessagePart represents a single message/content from a DATA transmission
from an SMTP client. This contains the headers and body content. It also contains
a reference to a collection of sub-messages, if any. This allows us to support
the recursive tree-like nature of the MIME protocol.
*/
type ISMTPMessagePart interface {
	AddBody(body string) error
	AddHeaders(headerSet ISet) error
	BuildMessages(body string) error
	ContentIsMultipart() (bool, error)
	GetBody() string
	GetBoundary() (string, error)
	GetBoundaryFromHeaderString(header string) (string, error)
	GetContentDisposition() string
	GetContentType() string
	GetFilenameFromContentDisposition() string
	GetHeader(key string) string
	GetMessageParts() []ISMTPMessagePart
	ParseMessages(body string, boundary string) error
}
