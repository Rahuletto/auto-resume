# models.py
from pydantic import BaseModel
from typing import List, Optional

class LinkedinDateInfo(BaseModel):
    year: int
    month: int
    day: int

class LinkedinGeo(BaseModel):
    country: str
    city: str
    full: str

class LinkedinEducation(BaseModel):
    start: LinkedinDateInfo
    end: LinkedinDateInfo
    fieldOfStudy: str
    degree: str
    grade: str
    schoolName: str
    description: str
    activities: str
    url: str
    schoolId: str

class LinkedinPosition(BaseModel):
    companyName: str
    companyUsername: str
    companyURL: str
    companyLogo: str
    companyIndustry: str
    companyStaffCountRange: str
    title: str
    location: str
    description: str
    employmentType: str
    start: LinkedinDateInfo
    end: Optional[LinkedinDateInfo] = None

class LinkedinLocaleSupport(BaseModel):
    country: str
    language: str

class Language(BaseModel):
    name: str
    proficiency: str

class Skill(BaseModel):
    name: str
    proficiency: Optional[str] = None
    hasSkillAssessment: Optional[bool] = None

class Position(BaseModel):
    title: str
    companyName: str
    location: str
    description: str
    employmentType: Optional[str] = "Full-time"
    start: LinkedinDateInfo
    end: Optional[LinkedinDateInfo] = None

class Certification(BaseModel):
    name: str

class Project(BaseModel):
    title: str
    description: str
    start: LinkedinDateInfo
    end: Optional[LinkedinDateInfo] = None

class LinkedinProject(BaseModel):
    total: int
    items: Optional[List[Project]] = None


class LinkedinProfile(BaseModel):
    id: int
    urn: str
    firstName: str
    lastName: str
    educations: List[LinkedinEducation]
    username: str
    summary: str
    headline: str
    isOpenToWork: Optional[bool] = None
    isHiring: Optional[bool] = None
    languages: List[Language]
    skills: List[Skill]
    position: List[Position]
    certifications: List[Certification]
    projects: LinkedinProject

    @classmethod
    def parse_obj(cls, obj: dict):
        # Ensure languages, skills, positions, certifications, and projects are parsed correctly
        obj['languages'] = [Language(**lang) for lang in obj.get('languages', [])]
        obj['skills'] = [Skill(**skill) for skill in obj.get('skills', [])]
        obj['position'] = [Position(**pos) for pos in obj.get('position', [])]
        obj['educations'] = [LinkedinEducation(**edu) for edu in obj.get('educations', [])]
        obj['certifications'] = [Certification(**cert) for cert in obj.get('certifications', [])]
        obj['projects'] = LinkedinProject(**obj.get('projects', {}))
        return super().parse_obj(obj)