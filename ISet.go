package mailslurper

/*
An ISet is a set of header items. Most emails, bodies, and attachments have
more than one header to describe what the content is and how to handle it.
*/
type ISet interface {
	Get(headerName string) (IItem, error)
	ParseHeaderString(headers string) error
	ToMap() map[string][]string
	UnfoldHeaders(headers string) string
}
