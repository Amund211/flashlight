package server

type HypixelAPIErrorResponse struct {
	Success bool   `json:"success"`
	Cause   string `json:"cause"`
}
