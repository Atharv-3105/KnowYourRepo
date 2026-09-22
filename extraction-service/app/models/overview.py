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
    # Optional - the LLM's own best guess at which real symbol this concept
    # relates to, if any (drawn from the representative_symbols it was
    # shown). Never trusted as a location by itself - the caller resolves
    # it against the real, currently-indexed symbol table before using it
    # as a citation, so a hallucinated or stale name just resolves to "no
    # citation" rather than a wrong one.
    symbol: str = ""


class GenerateOverviewResponse(BaseModel):
    narrative_summary: str
    concepts: list[OverviewConcept] = []
