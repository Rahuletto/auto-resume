package models

type LinkedinProfile struct {
	FirstName string       `json:"firstName"`
	LastName  string       `json:"lastName"`
	Username  string       `json:"username"`
	Position  []Experience `json:"position"`
	Projects  struct {
		Items []Project `json:"items"`
	} `json:"projects"`
	Certifications []Certification `json:"certifications"`
	Languages      []LanguageSkill `json:"languages"`
	Educations     []Education     `json:"educations"`
	Summary        string          `json:"summary"`
}

type Experience struct {
	Title       string `json:"title"`
	CompanyName string `json:"companyName"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Start       struct {
		Month int `json:"month"`
		Year  int `json:"year"`
	} `json:"start"`
	End struct {
		Month int `json:"month"`
		Year  int `json:"year"`
	} `json:"end"`
}

type Project struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Certification struct {
	Name string `json:"name"`
}

type LanguageSkill struct {
	Name        string `json:"name"`
	Proficiency string `json:"proficiency"`
}

type Education struct {
	SchoolName   string `json:"schoolName"`
	Degree       string `json:"degree"`
	FieldOfStudy string `json:"fieldOfStudy"`
	Grade        string `json:"grade"`
	Url          string `json:"url"`
	Start        struct {
		Month int `json:"month"`
		Year  int `json:"year"`
	} `json:"start"`
	End struct {
		Month int `json:"month"`
		Year  int `json:"year"`
	} `json:"end"`
}
