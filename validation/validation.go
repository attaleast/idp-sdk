// Package validation wraps go-playground/validator so validation failures
// come out as an *errors.Error with CodeInvalidArgument and a
// field-level Details map - ready to serialize straight into a HTTP 400
// response instead of hand-rolling that mapping in every handler
package validation

import (
	"fmt"

	sdkerrors "github.com/attaleast/idp-sdk/errors"
	"github.com/go-playground/validator/v10"
)

// Validator wraps *validator.Validate
type Validator struct {
	v *validator.Validate
}

// New builds a Validator using `validate` tags, e.g.:
//
//	type CreateProjectRequest struct {
//			Name string `json:"name" validate:"required,min=3,max=64"`
//			Slug string `json:"slug" validate:"required,alphanum"`
//	}
func New() *Validator {
	return &Validator{v: validator.New(validator.WithRequiredStructEnabled())}
}

// Struct validates s against its `validate` tags. On failure it returns
// an *errors.Error (CodeInvalidArgument) whose Details map is
// {field_name: human-readable reason}, safe to return directly to a
// client
func (val *Validator) Struct(s any) error {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}

	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return sdkerrors.Wrap(sdkerrors.CodeInvalidArgument, "validation failed", err)
	}

	details := make(map[string]any, len(verrs))
	for _, fe := range verrs {
		details[fe.Field()] = reason(fe)
	}
	return sdkerrors.New(sdkerrors.CodeInvalidArgument, "vaildation failed").WithDetails(details)
}

func reason(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "email":
		return "must be a valid email address"
	case "alphanum":
		return "must contain only letters and numbers"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	default:
		return fmt.Sprintf("falied validation: %s", fe.Tag())
	}
}
