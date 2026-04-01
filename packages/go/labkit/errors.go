package labkit

import (
	"errors"
	"fmt"
)

// ErrorClass groups errors by product responsibility.
type ErrorClass string

const (
	ErrorClassUser      ErrorClass = "user"
	ErrorClassSystem    ErrorClass = "system"
	ErrorClassEvaluator ErrorClass = "evaluator"
	ErrorClassAdmin     ErrorClass = "admin"
)

type classifiedError struct {
	class ErrorClass
	msg   string
	cause error
}

func (e *classifiedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

func (e *classifiedError) Unwrap() error {
	return e.cause
}

// Class returns the error class for this error.
func (e *classifiedError) Class() ErrorClass {
	return e.class
}

func NewUserError(msg string) error {
	return newClassifiedError(ErrorClassUser, msg, nil)
}

func WrapUserError(msg string, cause error) error {
	return newClassifiedError(ErrorClassUser, msg, cause)
}

func NewSystemError(msg string) error {
	return newClassifiedError(ErrorClassSystem, msg, nil)
}

func WrapSystemError(msg string, cause error) error {
	return newClassifiedError(ErrorClassSystem, msg, cause)
}

func NewEvaluatorError(msg string) error {
	return newClassifiedError(ErrorClassEvaluator, msg, nil)
}

func WrapEvaluatorError(msg string, cause error) error {
	return newClassifiedError(ErrorClassEvaluator, msg, cause)
}

func NewAdminError(msg string) error {
	return newClassifiedError(ErrorClassAdmin, msg, nil)
}

func WrapAdminError(msg string, cause error) error {
	return newClassifiedError(ErrorClassAdmin, msg, cause)
}

func newClassifiedError(class ErrorClass, msg string, cause error) error {
	return &classifiedError{class: class, msg: msg, cause: cause}
}

// ClassifyError returns the product class for known classified errors.
func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ""
	}
	var classified interface{ Class() ErrorClass }
	if errors.As(err, &classified) {
		return classified.Class()
	}
	return ErrorClassSystem
}
