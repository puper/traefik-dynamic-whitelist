package app

import "time"

type apiResponse struct {
	Result any       `json:"result"`
	Error  *apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type infoResult struct {
	CurrentIP    string           `json:"current_ip"`
	CurrentIPs   []string         `json:"current_ips"`
	TemporaryIPs []temporaryEntry `json:"temporary_ips"`
	PermanentIPs []permanentEntry `json:"permanent_ips"`
}

type temporaryEntry struct {
	IP        string    `json:"ip"`
	AddedAt   time.Time `json:"added_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type permanentEntry struct {
	IP      string    `json:"ip"`
	AddedAt time.Time `json:"added_at"`
}

type addRequest struct {
	Type string   `json:"type"`
	IP   string   `json:"ip"`
	IPs  []string `json:"ips"`
}

type deleteRequest struct {
	IP string `json:"ip"`
}
