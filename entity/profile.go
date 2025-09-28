package entity

type SimpleProfile struct {
	Name      string `json:"name"`
	NameAlias string `json:"name_alias"`
	Avatar    string `json:"avatar"`
}
