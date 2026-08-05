import openai

def summarize_all(documents):
    summaries = []
    for doc in documents:
        response = openai.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": f"Summarize: {doc}"}],
        )
        summaries.append(response.choices[0].message.content)
    return summaries
