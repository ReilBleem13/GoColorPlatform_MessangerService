package domain

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: msg,
		Status:  e.Status,
	}
}

var (
	ErrInvalidRequest = &AppError{
		Code:    "INVALID_REQUEST",
		Message: "Invalid request",
		Status:  400,
	}

	ErrInternalServerError = &AppError{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Internal server error",
		Status:  500,
	}

	ErrNotFound = &AppError{
		Code:    "NOT_FOUND",
		Message: "Not found",
		Status:  404,
	}

	///
	ErrAlreadyExists = &AppError{
		Code:    "ALREADY_EXISTS",
		Message: "Already exists",
		Status:  409,
	}
)
