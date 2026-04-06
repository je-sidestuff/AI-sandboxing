#!/usr/bin/env python3
"""Post 'REVISING: Begin revision for <command> at <timestamp>' PR comments.

Required environment variables:
  REVISE_INSTRUCTIONS_JSON - JSON array of instruction strings
  GITHUB_PAT               - GitHub personal access token
  REPO                     - Repository in 'owner/name' format
  PR_NUMBER                - Pull request number (as string)
"""

import json
import os
import urllib.error
import urllib.request
from datetime import datetime, timezone


def post_comment(repo, pr_number, body, pat):
    """Post a comment to a GitHub PR.

    Returns None if the repo or PR does not exist (404), allowing callers
    to treat non-existent repos/PRs gracefully.
    """
    url = f"https://api.github.com/repos/{repo}/issues/{pr_number}/comments"
    data = json.dumps({"body": body}).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={
            "Authorization": f"token {pat}",
            "Content-Type": "application/json",
            "Accept": "application/vnd.github.v3+json",
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return None
        raise


def main():
    instructions = json.loads(os.environ["REVISE_INSTRUCTIONS_JSON"])
    pat = os.environ["GITHUB_PAT"]
    repo = os.environ["REPO"]
    pr_number = os.environ["PR_NUMBER"]

    if not instructions:
        print("No REVISE instructions. Nothing to post.")
        return

    timestamp = datetime.now(tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    for instruction in instructions:
        body = f"REVISING: Begin revision for {instruction} at {timestamp}"
        result = post_comment(repo, pr_number, body, pat)
        if result is None:
            print(f"Skipped (repo/PR not found): {body[:80]}...")
        else:
            print(f"Posted comment: {body[:80]}...")


if __name__ == "__main__":
    main()
