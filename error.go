package walg

import (
	"fmt"
)

/**
 *  Used to indicate required environment variables for WAL-G.
 */
type UnsetEnvVarError struct {
	names []string
}

func (e UnsetEnvVarError) Error() string {
	msg := "Did not set the following environment variables:\n"
	for _, v := range e.names {
		msg = msg + v + "\n"
	}

	return msg
}

/**
 *  Used to signal no match found in string.
 */
type NoMatchAvailableError struct {
	str string
}

func (e NoMatchAvailableError) Error() string {
	msg := fmt.Sprintf("No match found in '%s'\n", e.str)
	return msg
}

/**
 *  Used to signal unsupported file types by WAL-G.
 */
type UnsupportedFileTypeError struct {
	Path       string
	FileFormat string
}

func (e UnsupportedFileTypeError) Error() string {
	msg := fmt.Sprintf("WAL-G does not support the file format %s in %s", e.FileFormat, e.Path)
	return msg
}