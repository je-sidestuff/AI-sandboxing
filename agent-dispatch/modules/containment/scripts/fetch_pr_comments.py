#!/usr/bin/env python3
"""Fetch PR comments and state from a GitHub PR using the GitHub REST API.

This script fetches PR comments and PR state (open/closed/merged) for use
by Terraform external data source. The native Terraform GitHub provider
data source does not expose merged_at or closed_at attributes, so we need
to fetch these via the REST API.

Terraform external data source interface:
  - Reads a JSON object from stdin with keys: pat, repo, pr_number
  - Writes a JSON object to stdout with keys:
    - comments_json: JSON-encoded map of ISO-8601 timestamp strings to comment body strings
    - revise_instructions_json: JSON-encoded list of REVISE: instruction strings
    - pr_state: raw GitHub state ("open" or "closed")
    - pr_merged: "true" or "false"
    - pr_merged_at: ISO-8601 timestamp or empty string
    - pr_closed_at: ISO-8601 timestamp or empty string
    - conclusion_state: simplified state ("active", "closed", or "merged")

Usage (standalone):
  echo '{"pat":"ghp_...","repo":"owner/repo","pr_number":"42"}' | python3 fetch_pr_comments.py
"""

import json
import sys
import urllib.request


def github_get_object(url: str, pat: str) -> dict:
    """Fetch a single object from a GitHub API endpoint."""
    req = urllib.request.Request(url, headers={
        "Authorization": f"Bearer {pat}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def github_get_list(url: str, pat: str) -> list:
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
    for item in github_get_list(issue_url, pat):
        comments[item["created_at"]] = item["body"]
    for item in github_get_list(review_url, pat):
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


def fetch_pr_state(pat: str, repo: str, pr_number: int) -> dict:
    """Fetch PR state including merged status from GitHub API."""
    url = f"https://api.github.com/repos/{repo}/pulls/{pr_number}"
    pr_data = github_get_object(url, pat)

    state = pr_data.get("state", "open")
    merged = pr_data.get("merged", False)
    merged_at = pr_data.get("merged_at") or ""
    closed_at = pr_data.get("closed_at") or ""

    # Compute simplified conclusion state
    if state == "open":
        conclusion_state = "active"
    elif merged:
        conclusion_state = "merged"
    else:
        conclusion_state = "closed"

    return {
        "pr_state": state,
        "pr_merged": "true" if merged else "false",
        "pr_merged_at": merged_at,
        "pr_closed_at": closed_at,
        "conclusion_state": conclusion_state,
    }


def main():
    query = json.load(sys.stdin)
    pat = query["pat"]
    repo = query["repo"]
    pr_number = int(query["pr_number"])

    comments = fetch_comments(pat, repo, pr_number)
    revise_instructions = extract_revise_instructions(comments)
    pr_state_info = fetch_pr_state(pat, repo, pr_number)

    # Terraform external data source requires a flat map(string) as output.
    # We JSON-encode the nested structures so callers can jsondecode() them back.
    output = {
        "comments_json": json.dumps(comments),
        "revise_instructions_json": json.dumps(revise_instructions),
    }
    output.update(pr_state_info)
    print(json.dumps(output))


if __name__ == "__main__":
    main()
