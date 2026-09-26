import logging

from fastapi import FastAPI
from prometheus_fastapi_instrumentator import Instrumentator

from app.auth import InternalKeyMiddleware
from app.config import settings
from app.routes.health import router as health_router
from app.routes.embed import router as embed_router
from app.routes.search import router as search_router
from app.routes.chat import router as chat_router
from app.routes.overview import router as overview_router
from app.middleware import RequestIDMiddleware

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")

if not settings.internal_api_key:
    logging.getLogger("auth").warning(
        "INTERNAL_API_KEY is not set - fine locally, but a publicly reachable sidecar must require it"
    )

# No CORS middleware on purpose: browsers never call this service - only the
# Go API does, server to server.
app = FastAPI(title="KnowYourRepo extraction service", docs_url=None, redoc_url=None)

app.include_router(health_router)
app.include_router(embed_router)
app.include_router(search_router)
app.include_router(chat_router)
app.include_router(overview_router)
app.add_middleware(InternalKeyMiddleware)
app.add_middleware(RequestIDMiddleware)

# Auto-instruments every route with request count/latency metrics and
# exposes them at /metrics - the standard, well-maintained way to get
# FastAPI HTTP metrics rather than hand-rolling middleware the way the Go
# side does. Custom LLM-router metrics (app/metrics.py) are separate,
# purpose-specific counters this library doesn't know about.
Instrumentator().instrument(app).expose(app)
