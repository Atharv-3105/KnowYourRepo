import asyncio
import sys
import warnings

if sys.platform == "win32":
    # psycopg's async mode needs SelectorEventLoop; Windows defaults to
    # ProactorEventLoop. This MUST run before uvicorn creates its event
    # loop - setting it inside app/main.py is too late, since the CLI
    # entrypoint (`python -m uvicorn ...`) already creates the loop via
    # asyncio.run() before app.main is ever imported. Windows-only dev
    # tech debt, dead code once this runs in a Linux container (Phase 6).
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", DeprecationWarning)
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())

import uvicorn

if __name__ == "__main__":
    uvicorn.run("app.main:app", host="0.0.0.0", port=8000, reload=True)