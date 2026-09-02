import logging 
import uuid 
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request

logger = logging.getLogger("request")

REQUEST_ID_HEADER = "X-Request-ID"

class RequestIDMiddleware(BaseHTTPMiddleware):
    """ 
        Reads X-Request-ID set by Go service(or generates one if it's missing when direct call using curl)
        logs request start/end with it, and echoes it back in the response header so logs can be correlated
    """
    
    async def dispatch(self, request: Request, call_next):
        request_id = request.headers.get(REQUEST_ID_HEADER) or str(uuid.uuid4())[:16]
        
        request.state.request_id = request_id
        
        logger.info("http_request_started request_id = %s method = %s path = %s", request_id, request.method, request.url.path)
        
        response = await call_next(request)
        
        response.headers[REQUEST_ID_HEADER] = request_id
        
        logger.info("http_request_completed request_id = %s method = %s path = %s status = %s", request_id, request.method, request.url.path, response.status_code)
        
        return response 