import os 
from dotenv import load_dotenv
from groq import Groq
from app.providers.base import LLMProvider

load_dotenv()

class GroqProvider(LLMProvider):
    
    def __init__(self):
        
        self.client = Groq(
            api_key = os.getenv("GROQ_API_KEY")
        )
        
        self.model = os.getenv("GROQ_MODEL", default="llama-3.3-70b-versatile")
        
    async def generate(self, prompt: str) -> str:
        
        response = self.client.chat.completions.create(
            model = self.model,
            messages=[
                {
                    "role":"system",
                    "content":
                        """ 
                            You are a senior software engineer.
                            Answer ONLY from the repository context.
                            
                            If the answer is not present in the context,
                            say so.
                        """
                },
                {
                    "role": "user",
                    "content": prompt,
                },
            ],
            temperature=0.1
        )
        
        return (response.choices[0].message.content)