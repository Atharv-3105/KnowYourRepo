from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.routes.health import router as health_router
from app.routes.embed import router as embed_router
from app.routes.search import router as search_router
from app.routes.chat import router as chat_router
from app.routes.classify import router as classify_router

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

