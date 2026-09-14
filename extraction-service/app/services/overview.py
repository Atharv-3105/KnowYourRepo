import json
import logging

from app.models.overview import GenerateOverviewResponse, OverviewConcept, OverviewEntrypoint
from app.providers.factory import get_llm_router

logger = logging.getLogger(__name__)
router = get_llm_router()


def _build_prompt(
    readme_text: str,
    entrypoints: list[OverviewEntrypoint],
    directory_structure: list[str],
    representative_symbols: list[str],
) -> str:
    readme_section = readme_text.strip() or "(no README found in this repository)"
    entrypoint_lines = "\n".join(f"- {e.name} ({e.file_path})" for e in entrypoints) or "(none detected)"
    dir_lines = "\n".join(f"- {d}" for d in directory_structure) or "(none)"
    symbol_lines = "\n".join(f"- {s}" for s in representative_symbols) or "(none)"

    return f"""You are helping a new engineer understand an unfamiliar codebase.

README
{readme_section}

Detected entrypoints
{entrypoint_lines}

Top-level directories
{dir_lines}

Representative symbols
{symbol_lines}

Respond with ONLY a single JSON object, no markdown code fences, no commentary, in exactly this shape:
{{"narrative_summary": "2-4 sentences describing what this project does and why, written for someone who has never seen it before", "concepts": [{{"term": "short name of a notable pattern, convention, or domain term used in this codebase", "explanation": "1-2 sentence explanation of it, specific to this codebase"}}]}}

Include at most 5 concepts. Only include concepts that are genuinely non-obvious - skip anything a competent engineer would already recognize on sight."""


def _parse_response(raw: str) -> GenerateOverviewResponse:
    text = raw.strip()
    if text.startswith("```"):
        text = text.strip("`")
        if text.lower().startswith("json"):
            text = text[4:]
        text = text.strip()

    try:
        parsed = json.loads(text)
    except json.JSONDecodeError as e:
        logger.warning("overview_generation_parse_failed error=%s raw=%s", e, raw[:500])
        raise ValueError(f"failed to parse overview generation response as JSON: {e}") from e

    if not isinstance(parsed, dict):
        logger.warning("overview_generation_parse_failed error=not a JSON object raw=%s", raw[:500])
        raise ValueError(
            f"expected overview generation response to be a JSON object, got {type(parsed).__name__}"
        )

    try:
        return GenerateOverviewResponse(
            narrative_summary=parsed.get("narrative_summary", ""),
            concepts=[OverviewConcept(**c) for c in parsed.get("concepts", [])],
        )
    except (TypeError, ValueError) as e:
        logger.warning("overview_generation_parse_failed error=%s raw=%s", e, raw[:500])
        raise ValueError(f"overview generation response had an unexpected shape: {e}") from e


async def generate_overview(
    readme_text: str,
    entrypoints: list[OverviewEntrypoint],
    directory_structure: list[str],
    representative_symbols: list[str],
) -> GenerateOverviewResponse:
    prompt = _build_prompt(readme_text, entrypoints, directory_structure, representative_symbols)
    raw = await router.complete(prompt, task_type="overview")
    return _parse_response(raw)
