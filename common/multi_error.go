package common

import "strings"

type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	s := []string{}
	for _, err := range m.Errors {
		s = append(s, err.Error())
	}

	return strings.Join(s, ", ")
}
