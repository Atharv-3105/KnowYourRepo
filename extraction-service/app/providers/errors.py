class ProviderError(Exception):
    """ 
        Raised by a LLM-Provider when a call fails
        
        rate_limited = True, tells the router to cool this provider down for 'retry_after' seconds and try a different one immediately,
        instead of treating it as a generic failure that counts towards the provider's consecutive-error trip
    """
    
    def __init__(self, message: str, rate_limited:bool = False, retry_after: int = 60):
        super().__init__(message)
        self.rate_limited = rate_limited 
        self.retry_after = retry_after
        
    