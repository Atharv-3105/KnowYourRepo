from fastapi import APIRouter

router = APIRouter(tags = ["meta"])

@router.get("/health")
async def health() -> dict:
    return {
        "status" : "ok", 
        "service" : "python microservice running"
    }