package mailslurper

/*
ISMTPMailItem is the interface that defines method for processing raw mail data
from an SMTP connection.
*/
type ISMTPMailItem interface {
	ProcessBody(streamInput string) error
	ProcessFrom(streamInput string) error
	ProcessRecipient(streamInput string) error
}
