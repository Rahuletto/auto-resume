# Generates a LaTeX resume from a template file and data fetched from GitHub and LinkedIn APIs
# The generated resume is saved to a file named resume.tex

import re
import requests
import os
import json
from dotenv import load_dotenv
from models.linkedin import *
from models.github import *
import calendar
from datetime import datetime

load_dotenv()

GITHUB_API_URL = "https://api.github.com/graphql"
GITHUB_TOKEN = os.getenv("TOKEN")

LINKEDIN_API_URL = "https://li-data-scraper.p.rapidapi.com/get-profile-data-by-url"
LINKEDIN_API_KEY = os.getenv("LINKEDIN_API_KEY")
LINKEDIN_PROFILE_URL = "https://linkedin.com/in/rahul-marban"

TEMPLATE_FILE = "misc/template.tex"
OUTPUT_FILE = "resume.tex"
GITHUB_DATA_FILE = "github_data.json"
LINKEDIN_DATA_FILE = "linkedin_data.json"
local = True

def cleanData(text):
    if not text:
        return ""
    return (text.replace("&", "\\&")
                .replace("%", "\\%")
                .replace("$", "\\$")
                .replace("#", "\\#")
                .replace("_", "\\_")
                .replace("{", "\\{")
                .replace("}", "\\}")
                .replace("\\", "\\textbackslash{}"))
                
class APIError(Exception):
    """Custom exception for API related errors"""
    pass

class FileOperationError(Exception):
    """Custom exception for file operation errors"""
    pass

class DataParsingError(Exception):
    """Custom exception for data parsing errors"""
    pass

def cleanData(data: str) -> str:
    """Clean and sanitize input data by removing non-ASCII characters and null bytes.

    Args:
        data (str): The input string to be cleaned.

    Returns:
        str: The cleaned string with only ASCII characters.

    Raises:
        DataParsingError: If the input is not a string or if cleaning fails.
    """
    try:
        if not isinstance(data, str):
            raise DataParsingError("Input must be a string, got {type(data)}")
        if not data.strip():
            return ""
        
        # Remove non-ASCII characters
        data = re.sub(r'[^\x00-\x7F]+', '', data)
        # Remove null bytes
        data = data.replace('\u0000', '')
        return data
    except Exception as e:
        raise DataParsingError(f"Error cleaning data: {str(e)}. Input type: {type(data)}")

def fetch_github_data(query: str) -> GithubResponse:
    """Fetch GitHub data using GraphQL API or load from local cache.

    Args:
        query (str): The GraphQL query to fetch GitHub data.

    Returns:
        GithubResponse: Parsed GitHub data response.

    Raises:
        APIError: If GitHub API request fails or token is missing.
        FileOperationError: If reading/writing local cache file fails.
        DataParsingError: If response parsing fails.
    """
    try:
        if local and os.path.exists(GITHUB_DATA_FILE):
            try:
                with open(GITHUB_DATA_FILE, "r", encoding='utf-8') as file:
                    data = json.load(file)
            except (IOError, json.JSONDecodeError) as e:
                raise FileOperationError(f"Failed to read GitHub cache file: {str(e)}")
        else:
            if not GITHUB_TOKEN:
                raise APIError("GitHub personal access token is not set in environment variables")
            
            headers = {
                "Authorization": f"Bearer {GITHUB_TOKEN}",
                "Content-Type": "application/json"
            }
            try:
                response = requests.post(GITHUB_API_URL, json={"query": query}, headers=headers, timeout=30)
                response.raise_for_status()
                data = response.json()
                
                if "errors" in data:
                    error_messages = [error.get('message', 'Unknown error') for error in data['errors']]
                    raise APIError(f"GitHub API returned errors: {', '.join(error_messages)}")
                
                if "data" not in data:
                    raise APIError("GitHub API response is missing 'data' field")
                    
                data = data["data"]
                
                if local:
                    try:
                        with open(GITHUB_DATA_FILE, "w", encoding='utf-8') as file:
                            json.dump(data, file, indent=2)
                    except IOError as e:
                        print(f"Warning: Failed to cache GitHub data: {str(e)}")
            except requests.exceptions.Timeout:
                raise APIError("GitHub API request timed out after 30 seconds")
            except requests.exceptions.RequestException as e:
                raise APIError(f"GitHub API request failed: {str(e)}")
        
        return GithubResponse.parse_obj(data)
    except Exception as e:
        if isinstance(e, (APIError, FileOperationError)):
            raise
        raise DataParsingError(f"Failed to process GitHub data: {str(e)}")

def fetch_linkedin_data() -> LinkedinProfile:
    """Fetch LinkedIn profile data using RapidAPI or load from local cache.

    Returns:
        LinkedinProfile: Parsed LinkedIn profile data.

    Raises:
        APIError: If LinkedIn API request fails or credentials are missing.
        FileOperationError: If reading/writing local cache file fails.
        DataParsingError: If response parsing fails.
    """
    try:
        if local and os.path.exists(LINKEDIN_DATA_FILE):
            try:
                with open(LINKEDIN_DATA_FILE, "r", encoding='utf-8') as file:
                    data = json.load(file)
            except (IOError, json.JSONDecodeError) as e:
                raise FileOperationError(f"Failed to read LinkedIn cache file: {str(e)}")
        else:
            if not LINKEDIN_API_KEY or not LINKEDIN_PROFILE_URL:
                raise APIError("LinkedIn API key or profile URL is not set in environment variables")
            
            headers = {
                "x-rapidapi-key": LINKEDIN_API_KEY,
                "x-rapidapi-host": "li-data-scraper.p.rapidapi.com",
            }
            params = {"url": LINKEDIN_PROFILE_URL}
            try:
                response = requests.get(LINKEDIN_API_URL, headers=headers, params=params, timeout=30)
                response.raise_for_status()
                data = response.json()
                
                if local:
                    try:
                        with open(LINKEDIN_DATA_FILE, "w", encoding='utf-8') as file:
                            json.dump(data, file, indent=2)
                    except IOError as e:
                        print(f"Warning: Failed to cache LinkedIn data: {str(e)}")
            except requests.exceptions.Timeout:
                raise APIError("LinkedIn API request timed out after 30 seconds")
            except requests.exceptions.RequestException as e:
                raise APIError(f"LinkedIn API request failed: {str(e)}")
        
        return LinkedinProfile.parse_obj(data)
    except Exception as e:
        if isinstance(e, (APIError, FileOperationError)):
            raise
        raise DataParsingError(f"Failed to process LinkedIn data: {str(e)}")

def update_latex_template(data: GithubResponse, linkedin_data: LinkedinProfile) -> None:
    """Update LaTeX template with GitHub and LinkedIn data.

    Args:
        data (GithubResponse): GitHub data containing user info and repositories.
        linkedin_data (LinkedinProfile): LinkedIn profile data.

    Raises:
        FileOperationError: If template file cannot be read or written.
        DataParsingError: If data processing or template updating fails.
    """
    try:
        try:
            with open(TEMPLATE_FILE, "r", encoding='utf-8') as template_file:
                template_content = template_file.read()
        except IOError as e:
            raise FileOperationError(f"Failed to read template file: {str(e)}")

        repositories = data.viewer.repositories
        repo_entries = []

        # Process top 3 repositories
        for repo in repositories[:3]:
            # Find matching project from LinkedIn data
            matching_project = next(
                (proj for proj in linkedin_data.projects.items 
                 if proj.title.lower() == repo.name.lower()),
                None
            )

            # Get project description and languages
            if matching_project:
                description_parts = matching_project.description.split('\n---\n')
                description = description_parts[0].strip()
                languages = description_parts[1].strip() if len(description_parts) > 1 else 'No languages available.'
                bullet_points = matching_project.description.split('- ')[1:]
            else:
                description = 'No description available.'
                languages = 'No languages available.'
                bullet_points = []

            # Build repository entry
            entry = [
                f"\\textbf{{\\href{{{repo.url}}}{{{repo.name}}}}} \\(\\mid\\) \\textbf{{{languages}}}",
                "\\begin{itemize}\n\\itemsep -3pt{}"
            ]

            # Add bullet points
            for point in bullet_points:
                point_text = point.strip().split("\n---\n")[0]
                point_text = re.sub(r'"([^"]+)"', r'\\textbf{\1}', point_text)
                entry.append(f"\\item {cleanData(point_text).replace('%', '\\%')}")
            entry.append("\\end{itemize}")
            repo_entries.append('\n'.join(entry))

        # Join all repository entries
        repo_entries = '\n'.join(repo_entries)
        languages = set() 
        for repo in repositories:
            for language in repo.languages:
                languages.add(language.name)

        github_languages = ", ".join(languages) if languages else ""

        experiences = linkedin_data.position[:3]
        certifications = linkedin_data.certifications
        speaks = linkedin_data.languages
        education = linkedin_data.educations
        
        def month_number_to_abbr(month_number: int) -> str:
            return calendar.month_abbr[month_number]
        
        current_year = datetime.now().year
        education_entries = "".join([
            f"\\textbf{{{cleanData(edu.degree.split(' - ')[1]).replace('B', 'B.').replace('M', 'M.')}}} {edu.fieldOfStudy} \\hfill {f'{month_number_to_abbr(edu.end.month)} {edu.end.year}' if edu.end and edu.end.year < current_year else f'Expected {edu.end.year}'}\\\\\n"
            f"\\href{{{edu.url}}}{{{cleanData(edu.schoolName.split(' (')[0])}}} \\hfill \\textit{{CGPA: {cleanData(edu.grade)}}}\n"
            for edu in education
        ])

        experience_entries = "".join([
            f"\\textbf{{{cleanData(exp.title)}}} \\hfill {month_number_to_abbr(exp.start.month)} {exp.start.year} - "
            f"{f'{month_number_to_abbr(exp.end.month)} {exp.end.year}' if exp.end and exp.end.year != 0 else 'Present'}\\\\\n"
            f"{cleanData('SIIC Chennai') if cleanData(exp.companyName) == 'SRM Innovation and Incubation Centre' else cleanData(exp.companyName)} \\hfill \\textit{{{cleanData(exp.location)}}}\n"
            + (f"\n{cleanData(exp.description.split('- ')[0])}\n"
            f"\\begin{{itemize}}\n\\itemsep -3pt{{}}\n"
            + "".join([f"\\item {cleanData(point.strip())}\n" for point in exp.description.replace("%", "\\%").split('- ')[1:]]) +
            f"\\end{{itemize}}\n"
            if "- " in exp.description else f"\n{cleanData(exp.description)}\n\n")
            for exp in experiences
        ])

        certification_entries = ", ".join([
            f"{cleanData(cert.name)}" for cert in certifications
        ])
        
        speaks_entries = ", ".join([
            f"{cleanData(speak.name)} ({cleanData(speak.proficiency.replace('PROFESSIONAL_WORKING', 'Professional').replace('ELEMENTARY', 'Elementary').replace('NATIVE_OR_BILINGUAL', 'Native'))})" for speak in speaks
        ])

        updated_content = template_content.replace("<REPOSITORIES>", repo_entries)
        updated_content = updated_content.replace("<EXPERIENCES>", experience_entries)
        updated_content = updated_content.replace("<EDUCATION>", education_entries)
        updated_content = updated_content.replace("<CERTIFICATIONS>", certification_entries)
        updated_content = updated_content.replace("<GITHUB_LANGS>", github_languages)
        updated_content = updated_content.replace("<SPEAKS>", speaks_entries)
        updated_content = updated_content.replace("<NAME>", linkedin_data.firstName + " " + linkedin_data.lastName)
        updated_content = updated_content.replace("<LOCATION>", data.viewer.location if data.viewer.location else "")
        updated_content = updated_content.replace("<EMAIL>", data.viewer.email if data.viewer.email else "")
        updated_content = updated_content.replace("<LINKEDIN>", f"linkedin.com/in/{linkedin_data.username}" if linkedin_data.username else "")
        updated_content = updated_content.replace("<LINKEDIN_TXT>", f"linkedin.com/in/{linkedin_data.username}" if linkedin_data.username else "")
        github_url = f"github.com/{data.viewer.login}" if data.viewer.login else ""
        updated_content = updated_content.replace("<GITHUB>", github_url)
        updated_content = updated_content.replace("<GITHUB_TXT>", f"github/{data.viewer.login}" if data.viewer.login else "")
        website_url = data.viewer.websiteUrl if data.viewer.websiteUrl else ""
        updated_content = updated_content.replace("<URL>", website_url.replace("https://", "").replace("http://", ""))
        # updated_content = updated_content.replace("<SUMMARY>", cleanData(linkedin_data.summary))

        with open(OUTPUT_FILE, "w") as output_file:
            output_file.write(cleanData(updated_content))

        print(f"LaTeX file updated: {OUTPUT_FILE}")
    except Exception as e:
        print(f"Error updating LaTeX template: {e}")
        raise e



query = """
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
}
"""

if __name__ == "__main__":
    try:
        github_data = fetch_github_data(query)
        linkedin_data = fetch_linkedin_data()
        update_latex_template(github_data, linkedin_data)
    except APIError as e:
        print(f"API Error: {str(e)}")
        exit(1)
    except FileOperationError as e:
        print(f"File Operation Error: {str(e)}")
        exit(1)
    except DataParsingError as e:
        print(f"Data Parsing Error: {str(e)}")
        exit(1)
    except Exception as e:
        print(f"Unexpected Error: {str(e)}")
        exit(1)
