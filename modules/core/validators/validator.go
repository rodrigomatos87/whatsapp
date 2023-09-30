package validators

import (
	"fmt"
	"ravi/models"
)

type validator struct {
}

func New() models.Validators {
	return &validator{}
}

func (v validator) String(value string, paramName string) error {
	if value == "" {
		return v.err(paramName)
	}

	return nil
}

func (v validator) Uint(value uint, paramName string) error {
	if value == 0 {
		return v.err(paramName)
	}

	return nil
}

func (v validator) err(paramName string) error {
	return fmt.Errorf("%s não pode ser vazio", paramName)
}
