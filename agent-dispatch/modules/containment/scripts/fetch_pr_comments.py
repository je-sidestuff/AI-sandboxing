#!/usr/bin/env python3
"""Fetch all comments from a GitHub PR using PyGithub.

Terraform external data source interface:
  - Reads a JSON object from stdin with keys: pat, repo, pr_number
  - Writes a JSON object to stdout with key: comments_json
    (a JSON-encoded list of comment body strings)

Usage (standalone):
  echo '{"pat":"ghp_...","repo":"owner/repo","pr_number":"42"}' | python3 fetch_pr_comments.py
"""

import json
import sys

from github import Github


def fetch_comments(pat: str, repo: str, pr_number: int) -> list[str]:
    """Return all comment bodies on the given PR (issue + review comments)."""
    g = Github(pat)
    repo_obj = g.get_repo(repo)
    pr = repo_obj.get_pull(pr_number)

    comments = []

    # General PR comments (issue comments on the PR thread)
    for comment in pr.get_issue_comments():
        comments.append(comment.body)

    # Inline review comments (on specific lines of code)
    for comment in pr.get_review_comments():
        comments.append(comment.body)

    return comments


def main():
    query = json.load(sys.stdin)
    pat = query["pat"]
    repo = query["repo"]
    pr_number = int(query["pr_number"])

    comments = fetch_comments(pat, repo, pr_number)

    # Terraform external data source requires a flat map(string) as output.
    # We JSON-encode the list so callers can jsondecode() it back.
    print(json.dumps({"comments_json": json.dumps(comments)}))


if __name__ == "__main__":
    main()
