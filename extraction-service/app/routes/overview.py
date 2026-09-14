from fastapi import APIRouter, HTTPException

from app.models.overview import GenerateOverviewRequest, GenerateOverviewResponse
from app.services.overview import generate_overview

router = APIRouter()


@router.post("/generate-overview", response_model=GenerateOverviewResponse)
async def generate_overview_route(request: GenerateOverviewRequest):
    try:
        return await generate_overview(
            request.readme_text,
            request.entrypoints,
            request.directory_structure,
            request.representative_symbols,
        )
    except ValueError as e:
        raise HTTPException(status_code=502, detail=str(e))
