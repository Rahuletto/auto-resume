package main

import (
	"autoresume/errors"
	"autoresume/models"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	// "github.com/joho/godotenv"
	"github.com/valyala/fasthttp"
)

const (
	GithubApiUrl       = "https://api.github.com/graphql"
	LinkedinApiUrl     = "https://li-data-scraper.p.rapidapi.com/get-profile-data-by-url"
	LinkedinProfileUrl = "https://linkedin.com/in/rahul-marban"
	TemplateFile       = "misc/template.tex"
	OutputFile         = "resume.tex"
	GithubDataFile     = "github_data.json"
	LinkedinDataFile   = "linkedin_data.json"
	PdfOutputFile      = "resume.pdf"
)

var (
	local         = true
	months        = [...]string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	nonAsciiRegex = regexp.MustCompile(`[^\x00-\x7F]+`)
)

func cleanData(data string) string {
	if len(strings.TrimSpace(data)) == 0 {
		return ""
	}

	cleanedData := strings.ReplaceAll(data, "&", "\\&")
	cleanedData = strings.ReplaceAll(cleanedData, "%", "\\%")

	return cleanedData
}

func monthNumberToAbbr(monthNumber int) string {
	if monthNumber < 1 || monthNumber > 12 {
		return ""
	}
	return months[monthNumber]
}

func fetchGithubData(query string) (*models.GithubResponse, error) {
	var data models.GithubResponse

	if local && fileExists(GithubDataFile) {
		fileData, err := os.ReadFile(GithubDataFile)
		if err != nil {
			return nil, errors.FileOperationError{Message: fmt.Sprintf("Failed to read GitHub cache file: %v", err)}
		}

		if err := json.Unmarshal(fileData, &data); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse GitHub cache file: %v", err)}
		}
	} else {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			return nil, errors.ApiError{Message: "GitHub personal access token is not set in environment variables"}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI(GithubApiUrl)
		req.Header.SetMethod("POST")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", githubToken))
		req.Header.SetContentType("application/json")

		queryBody := struct {
			Query string `json:"query"`
		}{
			Query: query,
		}

		bodyBytes, err := json.Marshal(queryBody)
		if err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to marshal GitHub query: %v", err)}
		}

		req.SetBody(bodyBytes)

		timeoutClient := &fasthttp.Client{
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
		}

		if err := timeoutClient.Do(req, resp); err != nil {
			return nil, errors.ApiError{Message: fmt.Sprintf("GitHub API request failed: %v", err)}
		}

		if resp.StatusCode() != fasthttp.StatusOK {
			return nil, errors.ApiError{Message: fmt.Sprintf("GitHub API returned non-OK status: %d", resp.StatusCode())}
		}

		var responseData struct {
			Data   models.GithubResponse `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := json.Unmarshal(resp.Body(), &responseData); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse GitHub API response: %v", err)}
		}

		if len(responseData.Errors) > 0 {
			errorMessages := make([]string, len(responseData.Errors))
			for i, err := range responseData.Errors {
				errorMessages[i] = err.Message
			}
			return nil, errors.ApiError{Message: fmt.Sprintf("GitHub API returned errors: %s", strings.Join(errorMessages, ", "))}
		}

		data = responseData.Data

		if local {
			cacheData, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				log.Printf("Warning: Failed to marshal GitHub data for caching: %v", err)
			} else {
				if err := os.WriteFile(GithubDataFile, cacheData, 0644); err != nil {
					log.Printf("Warning: Failed to cache GitHub data: %v", err)
				}
			}
		}
	}

	return &data, nil
}

func fetchLinkedinData() (*models.LinkedinProfile, error) {
	var data models.LinkedinProfile

	if local && fileExists(LinkedinDataFile) {
		fileData, err := os.ReadFile(LinkedinDataFile)
		if err != nil {
			return nil, errors.FileOperationError{Message: fmt.Sprintf("Failed to read LinkedIn cache file: %v", err)}
		}

		if err := json.Unmarshal(fileData, &data); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse LinkedIn cache file: %v", err)}
		}
	} else {
		linkedinApiKey := os.Getenv("LINKEDIN_API_KEY")
		if linkedinApiKey == "" {
			return nil, errors.ApiError{Message: "LinkedIn API key is not set in environment variables"}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI(fmt.Sprintf("%s?url=%s", LinkedinApiUrl, LinkedinProfileUrl))
		req.Header.SetMethod("GET")
		req.Header.Set("x-rapidapi-key", linkedinApiKey)
		req.Header.Set("x-rapidapi-host", "li-data-scraper.p.rapidapi.com")

		timeoutClient := &fasthttp.Client{
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
		}

		if err := timeoutClient.Do(req, resp); err != nil {
			return nil, errors.ApiError{Message: fmt.Sprintf("LinkedIn API request failed: %v", err)}
		}

		if resp.StatusCode() != fasthttp.StatusOK {
			return nil, errors.ApiError{Message: fmt.Sprintf("LinkedIn API returned non-OK status: %d", resp.StatusCode())}
		}

		if err := json.Unmarshal(resp.Body(), &data); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse LinkedIn API response: %v", err)}
		}

		if local {
			cacheData, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				log.Printf("Warning: Failed to marshal LinkedIn data for caching: %v", err)
			} else {
				if err := os.WriteFile(LinkedinDataFile, cacheData, 0644); err != nil {
					log.Printf("Warning: Failed to cache LinkedIn data: %v", err)
				}
			}
		}
	}

	return &data, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}

		log.Printf("Error checking file existence: %v", err)
		return false
	}
	return true
}

func updateLatexTemplate(githubData *models.GithubResponse, linkedinData *models.LinkedinProfile) error {
	templateContent, err := os.ReadFile(TemplateFile)
	if err != nil {
		return errors.FileOperationError{Message: fmt.Sprintf("Failed to read template file: %v", err)}
	}

	content := string(templateContent)
	repositories := githubData.Viewer.Repositories
	var repoEntries []string

	maxRepos := len(repositories.Nodes)
	if maxRepos > 3 {
		maxRepos = 3
	}

	for i := 0; i < maxRepos; i++ {
		repo := repositories.Nodes[i]

		var matchingProject *models.Project = nil
		for j := range linkedinData.Projects.Items {
			if strings.EqualFold(linkedinData.Projects.Items[j].Title, repo.Name) {
				matchingProject = &linkedinData.Projects.Items[j]
				break
			}
		}

		var languages string
		var bulletPoints []string

		if matchingProject != nil {
			descriptionParts := strings.Split(matchingProject.Description, "\n---\n")

			if len(descriptionParts) > 1 {
				languages = strings.TrimSpace(descriptionParts[1])
			} else {
				languages = "No languages available."
			}

			if parts := strings.Split(matchingProject.Description, "- "); len(parts) > 1 {
				bulletPoints = parts[1:]
			}
		} else {
			// description = "No description available."
			languages = "No languages available."
		}

		entry := []string{
			fmt.Sprintf("\\textbf{\\href{%s}{%s}} \\(\\mid\\) \\textbf{%s}", repo.Url, repo.Name, languages),
			"\\begin{itemize}\n\\itemsep -3pt{}",
		}

		for _, point := range bulletPoints {
			pointText := strings.TrimSpace(point)
			if strings.Contains(pointText, "\n---\n") {
				pointText = strings.Split(pointText, "\n---\n")[0]
			}

			re := regexp.MustCompile(`"([^"]+)"`)
			pointText = re.ReplaceAllString(pointText, "\\textbf{$1}")

			entry = append(entry, fmt.Sprintf("\\item %s", cleanData(pointText)))
		}

		entry = append(entry, "\\end{itemize}")
		repoEntries = append(repoEntries, strings.Join(entry, "\n"))
	}

	languageSet := make(map[string]bool)
	for _, repo := range repositories.Nodes {
		for _, lang := range repo.Languages.Nodes {
			languageSet[lang.Name] = true
		}
	}

	var languages []string
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	githubLanguages := strings.Join(languages, ", ")

	var experienceEntries []string
	var maxExperiences = len(linkedinData.Position)
	if maxExperiences > 4 {
		maxExperiences = 4
	}

	for i := 0; i < maxExperiences; i++ {
		exp := linkedinData.Position[i]

		startDate := fmt.Sprintf("%s %d", monthNumberToAbbr(exp.Start.Month), exp.Start.Year)
		var endDate string
		if exp.End.Year == 0 {
			endDate = "Present"
		} else {
			endDate = fmt.Sprintf("%s %d", monthNumberToAbbr(exp.End.Month), exp.End.Year)
		}

		companyName := cleanData(exp.CompanyName)
		if companyName == "SRM Innovation and Incubation Centre" {
			companyName = "SIIC Chennai"
		}

		entry := []string{
			fmt.Sprintf("\\textbf{%s} \\hfill %s - %s\\\\", cleanData(exp.Title), startDate, endDate),
			fmt.Sprintf("%s \\hfill \\textit{%s}", companyName, cleanData(exp.Location)),
		}

		if strings.Contains(exp.Description, "- ") {
			descParts := strings.Split(exp.Description, "- ")[:3]
			entry = append(entry, fmt.Sprintf("\n%s\n", cleanData(descParts[0])))
			entry = append(entry, "\\begin{itemize}\n\\itemsep -3pt{}")

			for _, point := range descParts[1:] {
				entry = append(entry, fmt.Sprintf("\\item %s", cleanData(strings.TrimSpace(point))))
			}

			entry = append(entry, "\\end{itemize}")
		} else {
			entry = append(entry, fmt.Sprintf("\n%s\n", cleanData(exp.Description)))
		}

		experienceEntries = append(experienceEntries, strings.Join(entry, "\n"))
	}

	var educationEntries []string
	for _, edu := range linkedinData.Educations {
		degreeParts := strings.Split(edu.Degree, " - ")
		degreeType := degreeParts[1]
		degreeType = strings.ReplaceAll(degreeType, "B", "B.")
		degreeType = strings.ReplaceAll(degreeType, "M", "M.")

		schoolNameParts := strings.Split(edu.SchoolName, " (")
		schoolName := schoolNameParts[0]

		var endDate string
		currentYear := time.Now().Year()
		if edu.End.Year < currentYear {
			endDate = fmt.Sprintf("%s %d", monthNumberToAbbr(edu.End.Month), edu.End.Year)
		} else {
			endDate = fmt.Sprintf("Expected %d", edu.End.Year)
		}

		entry := []string{
			fmt.Sprintf("\\textbf{%s} %s \\hfill %s\\\\", cleanData(degreeType), edu.FieldOfStudy, endDate),
			fmt.Sprintf("\\href{%s}{%s} \\hfill \\textit{CGPA: %s}", edu.Url, cleanData(schoolName), cleanData(edu.Grade)),
		}

		educationEntries = append(educationEntries, strings.Join(entry, "\n"))
	}

	var certificationEntries []string
	for _, cert := range linkedinData.Certifications {
		certificationEntries = append(certificationEntries, cleanData(cert.Name))
	}

	var speaksEntries []string
	for _, lang := range linkedinData.Languages {
		proficiency := lang.Proficiency
		proficiency = strings.ReplaceAll(proficiency, "PROFESSIONAL_WORKING", "Professional")
		proficiency = strings.ReplaceAll(proficiency, "ELEMENTARY", "Elementary")
		proficiency = strings.ReplaceAll(proficiency, "NATIVE_OR_BILINGUAL", "Native")

		speaksEntries = append(speaksEntries, fmt.Sprintf("%s (%s)", cleanData(lang.Name), cleanData(proficiency)))
	}

	content = strings.ReplaceAll(content, "<REPOSITORIES>", strings.Join(repoEntries, "\n"))
	content = strings.ReplaceAll(content, "<EXPERIENCES>", strings.Join(experienceEntries, "\n"))
	content = strings.ReplaceAll(content, "<EDUCATION>", strings.Join(educationEntries, "\n"))
	content = strings.ReplaceAll(content, "<CERTIFICATIONS>", strings.Join(certificationEntries, ", "))
	content = strings.ReplaceAll(content, "<GITHUB_LANGS>", githubLanguages)
	content = strings.ReplaceAll(content, "<SPEAKS>", strings.Join(speaksEntries, ", "))
	content = strings.ReplaceAll(content, "<NAME>", linkedinData.FirstName+" "+linkedinData.LastName)
	content = strings.ReplaceAll(content, "<LOCATION>", githubData.Viewer.Location)
	content = strings.ReplaceAll(content, "<EMAIL>", githubData.Viewer.Email)
	content = strings.ReplaceAll(content, "<LINKEDIN>", fmt.Sprintf("linkedin.com/in/%s", linkedinData.Username))
	content = strings.ReplaceAll(content, "<LINKEDIN_TXT>", fmt.Sprintf("linkedin.com/in/%s", linkedinData.Username))

	githubUrl := fmt.Sprintf("github.com/%s", githubData.Viewer.Login)
	content = strings.ReplaceAll(content, "<GITHUB>", githubUrl)
	content = strings.ReplaceAll(content, "<GITHUB_TXT>", fmt.Sprintf("github/%s", githubData.Viewer.Login))

	websiteUrl := githubData.Viewer.WebsiteUrl
	websiteUrl = strings.ReplaceAll(websiteUrl, "https://", "")
	websiteUrl = strings.ReplaceAll(websiteUrl, "http://", "")
	content = strings.ReplaceAll(content, "<URL>", websiteUrl)

	if err := os.WriteFile(OutputFile, []byte(content), 0644); err != nil {
		return errors.FileOperationError{Message: fmt.Sprintf("Failed to write output file: %v", err)}
	}

	fmt.Printf("LaTeX file updated: %s\n", OutputFile)
	return nil
}

func main() {
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatalf("Error loading .env file")
	// }

	var ghData *models.GithubResponse
	var lkData *models.LinkedinProfile
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		var err error
		ghData, err = fetchGithubData(`
		{
		  viewer {
		    login
		    name
		    location
		    websiteUrl
		    email
		    repositories(first: 100, orderBy: {field: STARGAZERS, direction: DESC}) {
		      nodes {
		        name
		        url
		        languages(first: 10) {
		          nodes {
		            name
		          }
		        }
		        stargazerCount
		      }
		    }
		  }
		}`)
		if err != nil {
			log.Fatalf("Failed to fetch GitHub data: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		lkData, err = fetchLinkedinData()
		if err != nil {
			log.Fatalf("Failed to fetch LinkedIn data: %v", err)
		}
	}()

	wg.Wait()

	updateLatexTemplate(ghData, lkData)
}
