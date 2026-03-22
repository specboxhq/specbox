package domain

import "errors"

var (
	ErrDocumentNotFound      = errors.New("document not found")
	ErrDocumentAlreadyExists = errors.New("document already exists")
	ErrEditNoMatch           = errors.New("text or pattern not found in document")
	ErrNthOutOfRange         = errors.New("Nth occurrence does not exist")
	ErrLineOutOfRange        = errors.New("line number out of range")
	ErrInvalidRegex          = errors.New("invalid regex pattern")
	ErrHeadingNotFound       = errors.New("heading not found in document")
	ErrNotACheckbox          = errors.New("line is not a markdown checkbox")
	ErrNotATable             = errors.New("line is not within a markdown table")
	ErrInvalidPath           = errors.New("invalid document path: must be relative, no '..' allowed")
)
