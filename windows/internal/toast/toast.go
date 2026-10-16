package toast

type Notification struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Silent  bool   `json:"silent,omitempty"`
}
