package main

import (
	"autoresume/models"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

const validLinkedinJSON = `{"firstName":"Rahul","lastName":"Marban","username":"rahul-marban","position":[{"title":"Software Engineer Intern","companyName":"Example","start":{"month":12,"year":2025}}],"educations":[{"schoolName":"SRM","end":{"year":2027}}]}`

func TestLinkedinMissingKeyUsesSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("LINKEDIN_API_KEY", "")
	oldLocal := local
	local = false
	t.Cleanup(func() { local = oldLocal })
	if err := os.WriteFile(filepath.Join(".", LinkedinDataFile), []byte(validLinkedinJSON), 0644); err != nil {
		t.Fatal(err)
	}
	profile, err := fetchLinkedinData()
	if err != nil {
		t.Fatalf("valid snapshot should survive a missing API key: %v", err)
	}
	if profile.FirstName != "Rahul" || len(profile.Position) != 1 {
		t.Fatalf("snapshot data lost: %+v", profile)
	}
}

func TestLinkedinAPIFailurePreservesSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"rate limited", 429, `{"message":"quota exceeded"}`},
		{"provider error with HTTP 200", 200, `{"message":"profile unavailable"}`},
		{"empty profile", 200, `{}`},
		{"null profile", 200, `null`},
		{"invalid JSON", 200, `<html>upstream failure</html>`},
		{"partial profile", 200, `{"firstName":"Rahul","lastName":"Marban","username":"rahul-marban"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "linkedin_data.json")
			if err := os.WriteFile(path, []byte(validLinkedinJSON), 0644); err != nil {
				t.Fatal(err)
			}
			data, err := fetchLinkedinDataFrom(server.URL, path, "test-key", &fasthttp.Client{})
			if err != nil || data == nil || data.FirstName != "Rahul" {
				t.Fatalf("fallback failed: %v, %+v", err, data)
			}
			body, err := os.ReadFile(path)
			if err != nil || string(body) != validLinkedinJSON {
				t.Fatalf("bad response overwrote snapshot: %v", err)
			}
		})
	}
}

func TestLinkedinLiveDataRefreshesSnapshot(t *testing.T) {
	liveJSON := strings.Replace(validLinkedinJSON, "Example", "Updated Company", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != LinkedinProfileUrl || r.Header.Get("x-rapidapi-key") != "test-key" || r.Header.Get("x-rapidapi-host") != "professional-network-data.p.rapidapi.com" {
			t.Error("incorrect LinkedIn API request")
		}
		_, _ = w.Write([]byte(liveJSON))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "linkedin_data.json")
	if err := os.WriteFile(path, []byte(validLinkedinJSON), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := fetchLinkedinDataFrom(server.URL, path, "test-key", &fasthttp.Client{})
	if err != nil || data.Position[0].CompanyName != "Updated Company" {
		t.Fatalf("live data not used: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), "Updated Company") {
		t.Fatalf("snapshot not refreshed: %v", err)
	}
}

func TestLinkedinTransportFailureUsesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	path := filepath.Join(t.TempDir(), "linkedin_data.json")
	if err := os.WriteFile(path, []byte(validLinkedinJSON), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := fetchLinkedinDataFrom(server.URL, path, "test-key", &fasthttp.Client{ReadTimeout: time.Second, WriteTimeout: time.Second})
	if err != nil || data == nil {
		t.Fatalf("network failure lost snapshot: %v", err)
	}
}

func TestLinkedinNoValidSourceFails(t *testing.T) {
	for _, body := range []string{"", `{}`, `null`, `broken`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "linkedin_data.json")
			if body != "" {
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			data, err := fetchLinkedinDataFrom("http://unused.invalid", path, "", &fasthttp.Client{})
			if err == nil || data != nil {
				t.Fatal("must fail instead of publishing an empty resume")
			}
		})
	}
}

func TestCommittedLinkedinSnapshot(t *testing.T) {
	body, err := os.ReadFile(LinkedinDataFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseLinkedinProfile(body); err != nil {
		t.Fatal(err)
	}
}

func TestFallbackGeneratesBothResumes(t *testing.T) {
	body := []byte(validLinkedinJSON)
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	t.Setenv("LINKEDIN_API_KEY", "")
	if err := os.WriteFile(LinkedinDataFile, body, 0644); err != nil {
		t.Fatal(err)
	}
	profile, err := fetchLinkedinData()
	if err != nil {
		t.Fatal(err)
	}
	github := &models.GithubResponse{}
	github.Viewer.Login = "Rahuletto"
	for _, name := range []string{"manic", "rocket", "ClassPro", "bullet", "lavalamp", "agent-orchestrator"} {
		github.Viewer.Repositories.Nodes = append(github.Viewer.Repositories.Nodes, models.Repository{Name: name, Url: "https://github.com/Rahuletto/" + name})
	}
	for _, tc := range []struct{ template, output, pattern string }{
		{TemplateFile, OutputFile, `(?i)^(manic|classpro|rocket)$`},
		{AiTemplateFile, AiOutputFile, `(?i)^(agent-orchestrator|lavalamp|bullet)$`},
	} {
		if err := updateLatexTemplate(filepath.Join(root, tc.template), tc.output, regexp.MustCompile(tc.pattern), github, profile); err != nil {
			t.Fatal(err)
		}
		generated, err := os.ReadFile(tc.output)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`\name{Rahul Marban}`, "Example", "SRM", "2027", "rahul-marban"} {
			if !strings.Contains(string(generated), want) {
				t.Errorf("%s missing %q", tc.output, want)
			}
		}
		if strings.Contains(string(generated), "<EXPERIENCES>") || strings.Contains(string(generated), "<EDUCATION>") {
			t.Errorf("%s has unfilled sections", tc.output)
		}
	}
}
