package mailslurper

/*
IItem represents a single header entry. Headers describe emails, bodies,
and attachments. They are in the form of "Key: Value".
*/
type IItem interface {
	GetKey() string
	GetValues() []string
	ParseHeaderString(header string) error
}
