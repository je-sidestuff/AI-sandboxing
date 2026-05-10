
Similar:
- We will scan repos that may or may not exist for (branches that may or may not exist?)
- We will trigger executions based on PR comments
- Executable content may be triggered via PR comment content - but will now be contextualized in the broader context of the assignment

Different:
- We will now maintain an irreversible 'ledger' or interactions executed
  - The ledger may be in filesystem or gitfile medium
- The individual executions are all under a blanket of an 'assignmentsl'
- We will call the golang content, sometimes wrapped with python, but will avoid bash

Initial iteration - we will use details from this description throughout the boostrabbing but will only complete a minimal increment here:
- We will populate our initial execution module, example, and test
- Our test will prove that we may use our example to execute a branch-isolation flow to create a new branch, create a slopspace, populate a writespace with a new test repo and add it to the slopspace, deploy the slopspace, execute changes, and return the slopspace and submit it for a PR review. Once the PR is submitted the test will merge it and we will verify the example sees the execution is complete.