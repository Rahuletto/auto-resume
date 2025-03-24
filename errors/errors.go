package errors

import "fmt"

type FileOperationError struct {
	Message string
}

func (e FileOperationError) Error() string {
	return fmt.Sprintf("File Operation Error: %s", e.Message)
}

type DataParsingError struct {
	Message string
}

func (e DataParsingError) Error() string {
	return fmt.Sprintf("Data Parsing Error: %s", e.Message)
}

type ApiError struct {
	Message string
}

func (e ApiError) Error() string {
	return fmt.Sprintf("API Error: %s", e.Message)
}
