import boto3

bedrock = boto3.client("bedrock-runtime")


def answer_questions(questions, index, embed):
    """Answer each question with a per-item vector search + LLM call.

    Both calls sit inside the loop, so cost scales linearly with the number
    of questions — exactly the pattern finops-guard flags.
    """
    answers = []
    for question in questions:
        # FG-007: one vector query per question instead of a batched query.
        matches = index.query(vector=embed(question), top_k=5)

        # FG-005: one Bedrock invocation per question instead of a batch.
        response = bedrock.invoke_model(
            modelId="anthropic.claude-3-5-sonnet",
            body={"question": question, "context": matches},
        )
        answers.append(response)
    return answers
