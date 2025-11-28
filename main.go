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

	"github.com/valyala/fasthttp"
)

const (
	GithubApiUrl       = "https://api.github.com/graphql"
	LinkedinApiUrl     = "https://professional-network-data.p.rapidapi.com/get-profile-data-by-url"
	LinkedinProfileUrl = "https://linkedin.com/in/rahul-marban"
	TemplateFile       = "misc/template.tex"
	OutputFile         = "resume.tex"
	GithubDataFile     = "github_data.json"
	LinkedinDataFile   = "linkedin_data.json"
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
	cleaned := strings.ReplaceAll(data, "&", "\\&")
	cleaned = strings.ReplaceAll(cleaned, "%", "\\%")
	return cleaned
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
		token := os.Getenv("TOKEN")
		if token == "" {
			return nil, errors.ApiError{Message: "GitHub personal access token is not set in environment variables"}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI(GithubApiUrl)
		req.Header.SetMethod("POST")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.SetContentType("application/json")

		body, _ := json.Marshal(map[string]string{"query": query})
		req.SetBody(body)

		client := &fasthttp.Client{ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
		if err := client.Do(req, resp); err != nil {
			return nil, errors.ApiError{Message: fmt.Sprintf("GitHub API request failed: %v", err)}
		}

		if resp.StatusCode() != fasthttp.StatusOK {
			return nil, errors.ApiError{Message: fmt.Sprintf("GitHub API returned non-OK status: %d", resp.StatusCode())}
		}

		var parsed struct {
			Data   models.GithubResponse `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := json.Unmarshal(resp.Body(), &parsed); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse GitHub API response: %v", err)}
		}

		if len(parsed.Errors) > 0 {
			msgs := make([]string, len(parsed.Errors))
			for i, e := range parsed.Errors {
				msgs[i] = e.Message
			}
			return nil, errors.ApiError{Message: strings.Join(msgs, ", ")}
		}

		data = parsed.Data

		if local {
			cache, _ := json.MarshalIndent(data, "", "  ")
			_ = os.WriteFile(GithubDataFile, cache, 0644)
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
		key := os.Getenv("LINKEDIN_API_KEY")
		if key == "" {
			return nil, errors.ApiError{Message: "LinkedIn API key is not set in environment variables"}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI(fmt.Sprintf("%s?url=%s", LinkedinApiUrl, LinkedinProfileUrl))
		req.Header.SetMethod("GET")
		req.Header.Set("x-rapidapi-key", key)
		req.Header.Set("x-rapidapi-host", "professional-network-data.p.rapidapi.com")

		client := &fasthttp.Client{ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
		if err := client.Do(req, resp); err != nil {
			return nil, errors.ApiError{Message: fmt.Sprintf("LinkedIn API request failed: %v", err)}
		}
		if resp.StatusCode() != fasthttp.StatusOK {
			return nil, errors.ApiError{Message: fmt.Sprintf("LinkedIn API returned non-OK status: %d", resp.StatusCode())}
		}

		if err := json.Unmarshal(resp.Body(), &data); err != nil {
			return nil, errors.DataParsingError{Message: fmt.Sprintf("Failed to parse LinkedIn API response: %v", err)}
		}

		if local {
			cache, _ := json.MarshalIndent(data, "", "  ")
			_ = os.WriteFile(LinkedinDataFile, cache, 0644)
		}
	}

	return &data, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func updateLatexTemplate(githubData *models.GithubResponse, linkedinData *models.LinkedinProfile) error {
	templateContent, err := os.ReadFile(TemplateFile)
	if err != nil {
		return errors.FileOperationError{Message: fmt.Sprintf("Failed to read template file: %v", err)}
	}

	content := string(templateContent)
	repositories := githubData.Viewer.Repositories
	var repoEntries []string

	// Filter specific repos to showcase
	showcasePattern := regexp.MustCompile(`(?i)^(manic|classpro|simply-djs)$`)

	for _, repo := range repositories.Nodes {
		if !showcasePattern.MatchString(repo.Name) {
			continue
		}

		var matchingProject *models.Project
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
			}
			if parts := strings.Split(matchingProject.Description, "- "); len(parts) > 1 {
				bulletPoints = parts[1:]
			}
		}

		entry := []string{
			fmt.Sprintf("\\textbf{\\href{%s}{%s}} \\(\\mid\\) \\textbf{%s}", repo.Url, repo.Name, languages),
		}

		if len(bulletPoints) > 0 {
			entry = append(entry, "\\begin{itemize}\n\\itemsep -3pt{}")
			for _, p := range bulletPoints {
				t := strings.TrimSpace(p)
				if strings.Contains(t, "\n---\n") {
					t = strings.Split(t, "\n---\n")[0]
				}
				re := regexp.MustCompile(`"([^"]+)"`)
				t = re.ReplaceAllString(t, "\\textbf{$1}")
				entry = append(entry, fmt.Sprintf("\\item %s", cleanData(t)))
			}
			entry = append(entry, "\\end{itemize}")
		}

		repoEntries = append(repoEntries, strings.Join(entry, "\n"))
	}

	// collect languages
	langSet := make(map[string]bool)
	for _, repo := range repositories.Nodes {
		for _, l := range repo.Languages.Nodes {
			langSet[l.Name] = true
		}
	}
	var langs []string
	for l := range langSet {
		langs = append(langs, l)
	}
	githubLanguages := strings.Join(langs, ", ")

	// experiences
	internPattern := regexp.MustCompile(`(?i)intern`)

	var experienceEntries []string
	count := 0
	
	for _, exp := range linkedinData.Position {
		if internPattern.MatchString(exp.Title) {
			continue
		}
	
		start := fmt.Sprintf("%s %d", monthNumberToAbbr(exp.Start.Month),
			exp.Start.Year)
		end := "Present"
		if exp.End.Year != 0 {
			end = fmt.Sprintf("%s %d", monthNumberToAbbr(exp.End.Month),
				exp.End.Year)
		}
	
		entry := []string{
			fmt.Sprintf("\\textbf{%s} \\hfill %s - %s\\\\",
				cleanData(exp.Title), start, end),
			fmt.Sprintf("%s \\hfill \\textit{%s}",
				cleanData(exp.CompanyName), cleanData(exp.Location)),
			fmt.Sprintf("\n%s\n", cleanData(exp.Description)),
		}
	
		experienceEntries = append(experienceEntries,
			strings.Join(entry, "\n"))
	
		count++
	
		if count == 4 {
			break
		}
	}


	// educations
	var educationEntries []string
	for _, edu := range linkedinData.Educations {
		degreeParts := strings.Split(edu.Degree, " - ")
		degreeType := degreeParts[len(degreeParts)-1]
		schoolName := strings.Split(edu.SchoolName, " (")[0]
		end := fmt.Sprintf("Expected %d", edu.End.Year)
		entry := []string{
			fmt.Sprintf("\\href{%s}{%s} \\hfill %s\\\\", edu.Url, cleanData(schoolName), end),
			fmt.Sprintf("\\textbf{%s} %s \\hfill \\textit{CGPA: %s}",
				cleanData(degreeType), edu.FieldOfStudy, cleanData(edu.Grade)),
		}
		educationEntries = append(educationEntries, strings.Join(entry, "\n"))
	}

	var certEntries []string
	for _, c := range linkedinData.Certifications {
		certEntries = append(certEntries, cleanData(c.Name))
	}

	var speaks []string
	for _, l := range linkedinData.Languages {
		p := strings.ReplaceAll(l.Proficiency, "_", " ")
		speaks = append(speaks, fmt.Sprintf("%s (%s)", cleanData(l.Name), cleanData(p)))
	}

	content = strings.ReplaceAll(content, "<REPOSITORIES>", strings.Join(repoEntries, "\n"))
	content = strings.ReplaceAll(content, "<EXPERIENCES>", strings.Join(experienceEntries, "\n"))
	content = strings.ReplaceAll(content, "<EDUCATION>", strings.Join(educationEntries, "\n"))
	content = strings.ReplaceAll(content, "<CERTIFICATIONS>", strings.Join(certEntries, ", "))
	content = strings.ReplaceAll(content, "<GITHUB_LANGS>", githubLanguages)
	content = strings.ReplaceAll(content, "<SPEAKS>", strings.Join(speaks, ", "))
	content = strings.ReplaceAll(content, "<NAME>", linkedinData.FirstName+" "+linkedinData.LastName)
	content = strings.ReplaceAll(content, "<LOCATION>", githubData.Viewer.Location)
	content = strings.ReplaceAll(content, "<EMAIL>", githubData.Viewer.Email)
	content = strings.ReplaceAll(content, "<LINKEDIN>", fmt.Sprintf("linkedin.com/in/%s", linkedinData.Username))
	content = strings.ReplaceAll(content, "<LINKEDIN_TXT>", fmt.Sprintf("linkedin.com/in/%s", linkedinData.Username))
	githubUrl := fmt.Sprintf("github.com/%s", githubData.Viewer.Login)
	content = strings.ReplaceAll(content, "<GITHUB>", githubUrl)
	content = strings.ReplaceAll(content, "<GITHUB_TXT>", githubUrl)
	site := strings.ReplaceAll(githubData.Viewer.WebsiteUrl, "https://", "")
	content = strings.ReplaceAll(content, "<URL>", site)

	if err := os.WriteFile(OutputFile, []byte(content), 0644); err != nil {
		return errors.FileOperationError{Message: fmt.Sprintf("Failed to write output file: %v", err)}
	}
	fmt.Printf("LaTeX file updated: %s\n", OutputFile)
	return nil
}

func main() {
	var ghData *models.GithubResponse
	var lkData *models.LinkedinProfile
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		gh, err := fetchGithubData(`
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
		          nodes { name }
		        }
		        stargazerCount
		      }
		    }
		  }
		}`)
		if err != nil {
			log.Fatalf("GitHub fetch failed: %v", err)
		}
		ghData = gh
	}()
	go func() {
		defer wg.Done()
		lk, err := fetchLinkedinData()
		if err != nil {
			log.Fatalf("LinkedIn fetch failed: %v", err)
		}
		lkData = lk
	}()
	wg.Wait()

	updateLatexTemplate(ghData, lkData)
}
