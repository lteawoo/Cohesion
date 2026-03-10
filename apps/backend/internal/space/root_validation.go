package space

import (
	"errors"
	"io"
	"os"

	"taeu.kr/cohesion/internal/browse"
)

type SpaceRootValidationCode string

const (
	SpaceRootValidationCodeValid            SpaceRootValidationCode = "valid"
	SpaceRootValidationCodeNotFound         SpaceRootValidationCode = "not_found"
	SpaceRootValidationCodeNotDirectory     SpaceRootValidationCode = "not_directory"
	SpaceRootValidationCodePermissionDenied SpaceRootValidationCode = "permission_denied"
)

type ValidateSpaceRootRequest struct {
	SpacePath string `json:"space_path"`
}

func (req *ValidateSpaceRootRequest) Validate() error {
	if req == nil {
		return errors.New("request is required")
	}
	if req.SpacePath == "" {
		return errors.New("space_path is required")
	}
	return nil
}

type SpaceRootValidationResult struct {
	Valid   bool                    `json:"valid"`
	Code    SpaceRootValidationCode `json:"code"`
	Message string                  `json:"message,omitempty"`
}

type SpaceRootValidationError struct {
	result SpaceRootValidationResult
}

func NewSpaceRootValidationError(result SpaceRootValidationResult) *SpaceRootValidationError {
	return &SpaceRootValidationError{result: result}
}

func (e *SpaceRootValidationError) Error() string {
	return e.result.Message
}

func (e *SpaceRootValidationError) Result() SpaceRootValidationResult {
	return e.result
}

func ValidateSpaceRoot(path string) (SpaceRootValidationResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodeNotFound,
				Message: "Selected folder does not exist",
			}, nil
		}
		if browse.IsPermissionError(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodePermissionDenied,
				Message: "Selected folder is not readable by the server",
			}, nil
		}
		return SpaceRootValidationResult{}, err
	}

	if !info.IsDir() {
		return SpaceRootValidationResult{
			Valid:   false,
			Code:    SpaceRootValidationCodeNotDirectory,
			Message: "Selected path is not a directory",
		}, nil
	}

	dir, err := os.Open(path)
	if err != nil {
		if browse.IsPermissionError(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodePermissionDenied,
				Message: "Selected folder is not readable by the server",
			}, nil
		}
		if os.IsNotExist(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodeNotFound,
				Message: "Selected folder does not exist",
			}, nil
		}
		return SpaceRootValidationResult{}, err
	}
	defer dir.Close()

	if _, err := dir.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		if browse.IsPermissionError(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodePermissionDenied,
				Message: "Selected folder is not readable by the server",
			}, nil
		}
		if os.IsNotExist(err) {
			return SpaceRootValidationResult{
				Valid:   false,
				Code:    SpaceRootValidationCodeNotFound,
				Message: "Selected folder does not exist",
			}, nil
		}
		return SpaceRootValidationResult{}, err
	}

	return SpaceRootValidationResult{
		Valid:   true,
		Code:    SpaceRootValidationCodeValid,
		Message: "Selected folder is accessible",
	}, nil
}
