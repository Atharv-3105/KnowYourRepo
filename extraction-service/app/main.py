from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from prometheus_fastapi_instrumentator import Instrumentator

from app.routes.health import router as health_router
from app.routes.embed import router as embed_router
from app.routes.search import router as search_router
from app.routes.chat import router as chat_router
from app.routes.classify import router as classify_router
from app.middleware import RequestIDMiddleware

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins = ["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(health_router)
app.include_router(embed_router)
app.include_router(search_router)
app.include_router(chat_router)
app.include_router(classify_router)
app.add_middleware(RequestIDMiddleware)

# Auto-instruments every route with request count/latency metrics and
# exposes them at /metrics - the standard, well-maintained way to get
# FastAPI HTTP metrics rather than hand-rolling middleware the way the Go
# side does. Custom LLM-router metrics (app/metrics.py) are separate,
# purpose-specific counters this library doesn't know about.
Instrumentator().instrument(app).expose(app)