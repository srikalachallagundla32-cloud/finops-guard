import openai


def enrich_records(records):
    """Enrich each record with an LLM-generated summary."""
    enriched = []
    for record in records:
        # Loop-bound API call — one request per record, cost scales with N.
        response = openai.chat.completions.create(
            model="gpt-4o",
            messages=[
                {"role": "system", "content": "Summarize this record."},
                {"role": "user", "content": str(record)},
            ],
        )
        record["summary"] = response.choices[0].message.content
        enriched.append(record)
    return enriched
