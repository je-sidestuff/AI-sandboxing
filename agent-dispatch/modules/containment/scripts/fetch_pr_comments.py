#!/usr/bin/env python3
"""Fetch all comments from a GitHub PR using the GitHub REST API.

Terraform external data source interface:
  - Reads a JSON object from stdin with keys: pat, repo, pr_number
  - Writes a JSON object to stdout with key: comments_json
    (a JSON-encoded map of ISO-8601 timestamp strings to comment body strings)

Usage (standalone):
  echo '{"pat":"ghp_...","repo":"owner/repo","pr_number":"42"}' | python3 fetch_pr_comments.py
"""

import json
import sys
import urllib.request


def github_get(url: str, pat: str) -> list:
    """Fetch all pages from a GitHub API endpoint and return combined results."""
    results = []
    while url:
        req = urllib.request.Request(url, headers={
            "Authorization": f"Bearer {pat}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        })
        with urllib.request.urlopen(req) as resp:
            results.extend(json.loads(resp.read()))
            # Follow Link: <next_url>; rel="next" pagination
            link_header = resp.headers.get("Link", "")
            url = None
            for part in link_header.split(","):
                part = part.strip()
                if 'rel="next"' in part:
                    url = part.split(";")[0].strip().strip("<>")
                    break
    return results


def fetch_comments(pat: str, repo: str, pr_number: int) -> dict[str, str]:
    """Return all comment bodies on the given PR as a map of timestamp -> body."""
    base = f"https://api.github.com/repos/{repo}"
    issue_url = f"{base}/issues/{pr_number}/comments?per_page=100"
    review_url = f"{base}/pulls/{pr_number}/comments?per_page=100"

    comments = {}
    for item in github_get(issue_url, pat):
        comments[item["created_at"]] = item["body"]
    for item in github_get(review_url, pat):
        comments[item["created_at"]] = item["body"]
    return comments


def extract_revise_instructions(comments: dict[str, str]) -> list[str]:
    """Return the instruction text for any comment starting with 'REVISE:'."""
    instructions = []
    for body in comments.values():
        if body.startswith("REVISE:"):
            instruction = body[len("REVISE:"):].strip()
            if instruction:
                instructions.append(instruction)
    return instructions


def main():
    query = json.load(sys.stdin)
    pat = query["pat"]
    repo = query["repo"]
    pr_number = int(query["pr_number"])

    comments = fetch_comments(pat, repo, pr_number)
    revise_instructions = extract_revise_instructions(comments)

    # Terraform external data source requires a flat map(string) as output.
    # We JSON-encode the nested structures so callers can jsondecode() them back.
    print(json.dumps({
        "comments_json": json.dumps(comments),
        "revise_instructions_json": json.dumps(revise_instructions),
    }))


if __name__ == "__main__":
    main()
