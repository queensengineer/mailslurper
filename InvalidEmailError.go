// NOTE: Implement https://godoc.org/github.com/pkg/errors
package mailslurper

import "fmt"

/*
An InvalidEmailError is used to alert a client that an email address is invalid
*/
type InvalidEmailError struct {
	Email string
}

/*
InvalidEmail returns a new error object
*/
func InvalidEmail(email string) *InvalidEmailError {
	return &InvalidEmailError{
		Email: email,
	}
}

func (err *InvalidEmailError) Error() string {
	return fmt.Sprintf("The provided email address, '%s', is invalid", err.Email)
}
