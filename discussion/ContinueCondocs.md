# Additional Capabilities - Revert and Substeps.

## Substeps

Substeps are a group of one or more iterations that occur within a step. They follow the same paradigm as steps in that they nest and enter a deeper scope.

They are different in that they pass the parent step-file as well as the substep file to the invoked agent. The purpose of Substeps is to be able to perform boring or messy tasks without adding too much noise to the narrative of the outer step.

From the outer document we simply see the 'title' of the substep, the initial prompt is tucked away in the substep doc only. In the condoccer's navigation bar we see substeps alongside other types of iterations (revision A, substep B, etc). We may click on the substeps to auto-scroll (just like other iteration types) or we may enter by double clicking or with a widget on the right hand side of the selection cell. Just like other nav elements we may navigate up to climb back to the parent step.