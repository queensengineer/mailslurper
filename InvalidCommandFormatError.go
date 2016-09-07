// NOTE: Implement https://godoc.org/github.com/pkg/errors
package mailslurper

import "fmt"

/*
An InvalidCommandFormatError is used to alert a client that the command passed in
has an invalid format
*/
type InvalidCommandFormatError struct {
	InvalidCommand string
}

/*
InvalidCommandFormat returns a new error object
*/
func InvalidCommandFormat(command string) *InvalidCommandFormatError {
	return &InvalidCommandFormatError{
		InvalidCommand: command,
	}
}

func (err *InvalidCommandFormatError) Error() string {
	return fmt.Sprintf("%s command format is invalid", err.InvalidCommand)
}
