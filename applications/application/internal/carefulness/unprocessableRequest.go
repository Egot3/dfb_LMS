package carefulness

import (
	"encoding/json"
	"errors"
)

type UnprocessableRequest struct {
}

var ErrUnprocessableRequest UnprocessableRequest

func (e UnprocessableRequest) Error() string {
	return "Provided JSON is badly typed"
}

func (e UnprocessableRequest) JSONError() JSONError {
	return JSONError{Error: e.Error()}
}

func (e UnprocessableRequest) Is(target error) bool {
	var err *json.UnsupportedTypeError
	return errors.As(target, &err)
}
