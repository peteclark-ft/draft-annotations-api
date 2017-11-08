package concept

type Concept struct {
	Id         string   `json:"id"`
	ApiUrl     string   `json:"apiUrl,omitempty"`
	Types      []string `json:"types,omitempty"`
	PrefLabel  string   `json:"prefLabel,omitempty"`
	IsFTAuthor bool     `json:"isFTAuthor,omitempty"`
}