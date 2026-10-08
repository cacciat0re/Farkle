package models

type table struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MaxPlayer int    `json:"max_player"`
}
