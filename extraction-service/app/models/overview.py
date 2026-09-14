from pydantic import BaseModel


class OverviewEntrypoint(BaseModel):
    name: str
    file_path: str
    language: str


class GenerateOverviewRequest(BaseModel):
    readme_text: str = ""
    entrypoints: list[OverviewEntrypoint] = []
    directory_structure: list[str] = []
    representative_symbols: list[str] = []


class OverviewConcept(BaseModel):
    term: str
    explanation: str


class GenerateOverviewResponse(BaseModel):
    narrative_summary: str
    concepts: list[OverviewConcept] = []
