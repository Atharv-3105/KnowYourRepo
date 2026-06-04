from app.providers.groq_provider import GroqProvider
provider = GroqProvider()

async def generate_answer(context: str, question: str) -> str:
    
    prompt = f"""
    Repository Context
    
    {context}
    
    Question
    
    {question}
    
    Answer the question using only the repository context.
    """
    
    return await provider.generate(prompt)