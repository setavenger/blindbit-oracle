package server

func NewSuccessResponse(data any) *ApiResponse {
	return &ApiResponse{Success: true, Data: SuccessResponse{Data: data}}
}

func NewErrorResponse(error error, extra ...any) *ApiResponse {
	var errResp ErrorResponse
	if extra == nil {
		errResp = ErrorResponse{Error: error.Error()}
		return &ApiResponse{Success: false, Data: errResp}
	} else {
		errResp = ErrorResponse{Error: error.Error(), Extra: extra}
	}
	return &ApiResponse{Success: false, Data: errResp}
}

type ApiResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

type SuccessResponse struct {
	Data any `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Extra any    `json:"extra,omitempty"`
}
