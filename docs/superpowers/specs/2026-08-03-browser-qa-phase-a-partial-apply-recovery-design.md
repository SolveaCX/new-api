# Browser QA Phase A Partial-Apply Recovery Design

**Date:** 2026-08-03  
**Status:** Approved, implementation in progress  
**Environment:** Flatkey staging resources in `vocai-gemini-prod`  
**Predecessor:** [Browser QA Two-Phase Secret Bootstrap Design](./2026-08-03-browser-qa-two-phase-secret-bootstrap-design.md)

## Outcome

Recover the interrupted Browser QA Phase A apply without replaying the invalid saved plan, weakening the normal bootstrap guard, using `-target`, importing resources, or modifying any existing application service.

The recovery is intentionally incident-specific:

- Terraform state already contains exactly 23 of the 26 Phase A resources.
- A fresh refreshing plan with `create_workloads=false` contains exactly three creates, zero changes, and zero destroys.
- The three creates are the report-bucket IAM members that failed when the operator lacked `storage.buckets.setIamPolicy`.
- The operator now has temporary project-level `roles/storage.admin`; it is sufficient for the recovery but should be removed after the three IAM members are applied and verified.

## Incident Snapshot

The only remaining resources are:

1. `google_storage_bucket_iam_member.browser_qa_cleanup_report_admin`
2. `google_storage_bucket_iam_member.browser_qa_deployer_report_viewer`
3. `google_storage_bucket_iam_member.browser_qa_runtime_report_creator`

They grant the dedicated Browser QA service accounts these bucket-scoped roles:

- cleanup: `roles/storage.objectAdmin`
- deployer: `roles/storage.objectViewer`
- runtime: `roles/storage.objectCreator`

The report bucket itself and the other 22 Phase A resources already exist in the isolated Browser QA state. Secret versions, GitHub Variables, and Phase B Cloud Run workloads remain absent and are outside this recovery apply.

## Approaches Considered

### 1. Exact recovery phase in the existing plan guard — selected

Add an `infra-recovery` contract that accepts exactly the three remaining resource creates and no meaningful output changes. Known Phase A no-ops remain allowed, while every other meaningful action is rejected.

This keeps the recovery reviewable, testable, and bound to the same saved-plan protections as the normal two-phase bootstrap.

### 2. Allow arbitrary Phase A create subsets — rejected

Relaxing `--phase infra` to accept any subset would make future partial plans appear valid without an incident-specific review. It would weaken the exact 26-create bootstrap invariant and make omissions harder to detect.

### 3. Reapply the old plan or grant IAM manually and import — rejected

The old saved plan was consumed by a partial apply and no longer represents current state. Direct `gcloud` IAM writes followed by import, an unsaved apply, or `-target` would bypass the reviewed Terraform state transition and introduce unnecessary reconciliation risk.

## Guard Contract

`scripts/browser_qa/terraform_plan_guard.py` gains a third explicit phase:

```text
--phase infra-recovery
```

The phase contract requires:

- the meaningful resource address set equals the three addresses in the incident snapshot;
- every meaningful resource action equals `create`;
- there are no meaningful output changes;
- unknown resource or output addresses fail even when marked `no-op`;
- update, delete, replace, deposed objects, malformed shapes, and duplicate JSON keys continue to fail;
- normal `infra` and `workloads` contracts remain unchanged.

The recovery contract stays in the guard as an audited incident recovery path. It cannot be reused after success because a no-change plan fails with `saved plan has no resource changes`.

## Operator Flow

The recovery section in `deploy/gcp/docs/OPERATIONS.md` must execute these gates in order:

1. Confirm the active account, project `vocai-gemini-prod`, and region `us-west1`.
2. Confirm the previous saved plan is invalid and will not be reused.
3. Confirm the isolated Browser QA state address set is exactly the expected 23-resource pre-recovery set.
4. Generate a new refreshing saved plan with `create_workloads=false` in a new temporary review directory.
5. Export JSON and human-readable representations from that same saved plan.
6. Run `terraform_plan_guard.py --phase infra-recovery` against the JSON.
7. Confirm the human-readable summary is exactly `3 to add, 0 to change, 0 to destroy`.
8. Require a recovery-specific confirmation token before applying the same saved plan.
9. If apply returns nonzero, invalidate the plan again and stop.
10. After success, confirm Terraform state is exactly the full 26-resource Phase A set and the three bucket IAM members are present.
11. Remove the temporary project-level `roles/storage.admin` grant after the recovery is verified.
12. Resume at the existing Phase A outputs, GitHub Variables, Secret versions, and Phase B readiness steps.

The recovery flow continues to prohibit `-target`, `-refresh=false`, automatic import, manual deletion, and rollback into the production Terraform root.

## State Boundaries

This recovery changes only IAM policy membership on:

```text
gs://vocai-gemini-prod-flatkey-browser-qa-reports
```

It does not create or update:

- any GCP project;
- any existing staging or production Cloud Run service;
- any Cloud SQL instance;
- any Secret Manager version or payload;
- any GitHub Variable or Secret;
- any Browser QA Cloud Run service or job.

No Terraform HCL resource definition needs to change. The implementation is limited to the plan guard, its tests, the operations contract tests, and the runbook.

## Verification

Implementation is complete only when:

- a synthetic exact three-create recovery plan passes;
- missing or extra recovery resources fail;
- update, delete, replace, output mutation, deposed, duplicate-key, and unknown-address cases fail;
- normal 26-create Phase A and 9-create Phase B tests remain unchanged and pass;
- operations contract tests prove pre-state check, new saved-plan generation, guard invocation, confirmation token, invalid-plan handling, and post-state check ordering;
- all Browser QA tests pass;
- Terraform formatting and validation remain clean;
- a fresh real plan is exactly three creates, zero changes, and zero destroys and passes the new guard;
- no `terraform apply` is performed by the agent; the operator applies the reviewed saved plan from the manual terminal.
