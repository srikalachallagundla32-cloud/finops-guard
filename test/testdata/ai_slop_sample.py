import openai


def summarize_all(documents):
    results = []
    for doc in documents:
        response = openai.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": doc}],
        )
        results.append(response)
    return results
