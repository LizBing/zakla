package protocol

import (
	"encoding/json"
)

type StatusResponse struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`

	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
		Sample []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"sample"`
	} `json:"players"`

	Description struct {
		Text string `json:"text"`
	} `json:"description"`

	Favicon string `json:"favicon,omitempty"`
}

func (s *StatusResponse) Marshal() string {
	data, _ := json.Marshal(s)
	return string(data)
}
