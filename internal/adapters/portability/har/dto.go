package har

type harFile struct {
	Log struct {
		Version *string `json:"version"`
		Entries []entry `json:"entries"`
	} `json:"log"`
}

type entry struct {
	Request      harRequest   `json:"request"`
	Response     *harResponse `json:"response"`
	ResourceType string       `json:"_resourceType"`
}

type harRequest struct {
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Headers  []nameValue `json:"headers"`
	PostData *postData   `json:"postData"`
}

type nameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type postData struct {
	MimeType string  `json:"mimeType"`
	Text     string  `json:"text"`
	Params   []param `json:"params"`
}

type param struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	FileName string `json:"fileName"`
}

type harResponse struct {
	Status     int         `json:"status"`
	StatusText string      `json:"statusText"`
	Headers    []nameValue `json:"headers"`
	Content    content     `json:"content"`
}

type content struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding"`
}
