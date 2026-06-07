# Additional Capabilities - Revert and Substeps.

## Substeps

Substeps are a group of one or more iterations that occur within a step. They follow the same paradigm as steps in that they nest and enter a deeper scope.

They are different in that they pass the parent step-file as well as the substep file to the invoked agent. The purpose of Substeps is to be able to perform boring or messy tasks without adding too much noise to the narrative of the outer step.

From the outer document we simply see the 'title' of the substep, the initial prompt is tucked away in the substep doc only. In the condoccer's navigation bar we see substeps alongside other types of iterations (revision A, substep B, etc). We may click on the substeps to auto-scroll (just like other iteration types) or we may enter by double clicking or with a widget on the right hand side of the selection cell. Just like other nav elements we may navigate up to climb back to the parent step.

Substeps have the same indexing pattern that steps do, where the initial invocation is 'letterless' within the context of the substep (although in this case the overall substep has a handle which has a letter index for the outer step). So overall a substep iterations could be though of as (step_step-itetation_substep-iteration) 3_C_B.

Substeps have the same revise/retry/complete consideration as steps (but they return to the step rather than the main condoc).

## Reverts

Reverting is a similar idea to retrying but a revert removes content from condocs/stepfiles and does not perform an automatic reinvocation. Conceptually we use reverting when we want to rethink a path more deeply and will have less use of the original attempt as context.

With a revert we may specify a point anywhere in the condoc and return there - for example, '!REVERT-3!' would take us back to the beginning of step 3, even if we were currently somewhere in step 5. The directive '!REVERT-4-B-D!' would take us to step 4, iteration B (substep iteration D). A revert directive is applicable to an outer condoc, a step file, or a substep file and can undo completion of a step or substep. (Once an overall condoc is complete this cannot be reversed)

Just like a retry, the revert will save the diff file before rewriting history. This leaves the information in a recoverable state but not at the forefront of our context.

The condoccer widgets reflect the ability to revert.