import time
import requests


def wait_for_job(job_id):
    # FG-009: hot poll loop — hammers the status endpoint with no sleep/backoff.
    while True:
        resp = requests.get(f"https://api.example.com/jobs/{job_id}")
        if resp.json()["state"] == "done":
            return resp.json()
        # (no delay between polls — this is the leak)


def wait_for_job_throttled(job_id):
    # SAFE: same loop, but backs off between polls — FG-009 must NOT fire.
    while True:
        resp = requests.get(f"https://api.example.com/jobs/{job_id}")
        if resp.json()["state"] == "done":
            return resp.json()
        time.sleep(5)
