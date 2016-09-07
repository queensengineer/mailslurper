// NOTE: Implement https://godoc.org/github.com/pkg/errors
package mailslurper

import "fmt"

/*
An InvalidCommandError is used to alert a client that the command passed in
is invalid.
*/
type InvalidCommandError struct {
	InvalidCommand string
}

/*
InvalidCommand returns a new error object
*/
func InvalidCommand(command string) *InvalidCommandError {
	return &InvalidCommandError{
		InvalidCommand: command,
	}
}

func (err *InvalidCommandError) Error() string {
	return fmt.Sprintf("Invalid command %s", err.InvalidCommand)
}
