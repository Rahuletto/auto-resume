package models

type GithubResponse struct {
	Viewer struct {
		Login        string `json:"login"`
		Name         string `json:"name"`
		Location     string `json:"location"`
		WebsiteUrl   string `json:"websiteUrl"`
		Email        string `json:"email"`
		Repositories struct {
			Nodes []Repository `json:"nodes"`
		} `json:"repositories"`
	} `json:"viewer"`
}

type Repository struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	Languages struct {
		Nodes []Language `json:"nodes"`
	} `json:"languages"`
	StargazerCount int `json:"stargazerCount"`
}

type Language struct {
	Name string `json:"name"`
}
