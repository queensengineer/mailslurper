package mailslurper

import (
	"strings"
)

/*
GetCommandValue splits an input by colon (:) and returns the right hand side.
If there isn't a split, or a missing colon, an InvalidCommandFormatError is
returned.
*/
func GetCommandValue(streamInput, command, delimiter string) (string, error) {
	split := strings.Split(streamInput, delimiter)

	if len(split) < 2 {
		return "", InvalidCommandFormat(command)
	}

	return strings.TrimSpace(strings.Join(split[1:], "")), nil
}
