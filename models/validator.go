package models

type Validators interface {
	String(value string, paramName string) error
	Uint(value uint, paramName string) error
}
