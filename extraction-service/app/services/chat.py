from app.providers.factory import get_llm_router
router = get_llm_router()

async def generate_answer(context: str, question: str) -> str:
    
    prompt = f"""
    Repository Context
    
    {context}
    
    Question
    
    {question}
    
    Answer the question using only the repository context.
    """
    
    return await router.complete(prompt, task_type = "answer")