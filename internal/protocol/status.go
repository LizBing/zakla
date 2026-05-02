package protocol

import (
	"encoding/json"
)

type statusResponse struct {
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

func StatusResponseStr() string {
	s := statusResponse{}
	s.Version.Name = "Zakla 26.1"
	s.Version.Protocol = 775
	s.Players.Max = 10
	s.Players.Online = 0
	s.Description.Text = "Have a nice day!"

	data, _ := json.Marshal(s)
	return string(data)
}
