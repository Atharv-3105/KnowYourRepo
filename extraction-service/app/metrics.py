from prometheus_client import Counter

# LLM provider-level metrics. These reuse the same success/rate_limited/error
# vocabulary as RouterProvider's state machine (see providers/router.py) so
# the metric labels map directly onto the router's own internal reasoning
# about a provider, rather than inventing a second classification scheme.
LLM_ROUTER_REQUESTS_TOTAL = Counter(
    "llm_router_requests_total",
    "Total LLM router completion attempts, labeled by provider and outcome (success/rate_limited/error).",
    ["provider", "outcome"],
)

LLM_ROUTER_TOKENS_TOTAL = Counter(
    "llm_router_tokens_total",
    "Total tokens consumed per LLM provider, only counted on successful completions.",
    ["provider"],
)

# Embedding provider requests. Free-tier embedding quotas are counted per
# request, so this is the number to watch: cache_hit is a query embedding
# served without spending one, quota_exhausted means the daily cap was hit.
EMBED_REQUESTS_TOTAL = Counter(
    "embed_requests_total",
    "Embedding provider requests, labeled by kind (query/document) and outcome.",
    ["kind", "outcome"],
)
