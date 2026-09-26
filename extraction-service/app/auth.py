import hmac

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse

from app.config import settings

INTERNAL_KEY_HEADER = "X-Internal-Key"

# Liveness and scrape endpoints stay open - they expose nothing and cost
# nothing. Everything else spends provider quota or touches the database.
PUBLIC_PATHS = {"/health", "/metrics"}


class InternalKeyMiddleware(BaseHTTPMiddleware):
    """
        The sidecar is only ever called by the Go API. On most free hosting
        tiers every service gets a public URL, so without this anyone who
        found it could spend the embedding/LLM quota directly. The Go API
        sends the shared secret (SIDECAR_API_KEY there, INTERNAL_API_KEY here)
        on every request.
    """

    async def dispatch(self, request: Request, call_next):
        expected = settings.internal_api_key
        if expected and request.url.path not in PUBLIC_PATHS:
            provided = request.headers.get(INTERNAL_KEY_HEADER, "")
            if not hmac.compare_digest(provided.encode(), expected.encode()):
                return JSONResponse(status_code=401, content={"detail": "unauthorized"})
        return await call_next(request)
