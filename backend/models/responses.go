package models

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type TemplateVariable struct {
	Name    string  `json:"name"`
	Default *string `json:"default,omitempty"`
}

type TemplateVariablesResponse struct {
	Variables []TemplateVariable `json:"variables"`
}
