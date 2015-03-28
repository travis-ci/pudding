package pudding

import "strings"

// MultiError contains a slice of errors and implements the error
// interface
type MultiError struct {
	Errors []error
}

// Error provides a string that is the combination of all errors in
// the internal error slice
func (m *MultiError) Error() string {
	s := []string{}
	for _, err := range m.Errors {
		s = append(s, err.Error())
	}

	return strings.Join(s, ", ")
}
