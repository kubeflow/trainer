---
name: approve-dependabot-prs
description: Approves open Dependabot pull requests in kubeflow/trainer whose CI is green by submitting a GitHub Approve review with the /lgtm and /approve Prow commands. Use when asked to approve, LGTM, or triage Dependabot or dependency-bump PRs.
allowed-tools: ["Bash"]
---

# Approve Dependabot PRs

Find open Dependabot PRs in `kubeflow/trainer`, decide which ones are safe to approve, and approve
them with a GitHub **Approve** review whose body holds the Prow commands. Prow reads `/lgtm` and
`/approve` from the review body, adds the `lgtm` and `approved` labels, and Tide merges the PR.

Only PRs that meet **every** rule in [When to Approve](#when-to-approve) are approved. The user
must not have to recheck the PRs you approve, so when in doubt, skip the PR and report why.

## When to Approve

Approve a PR only if all of the following are true:

1. The author is `app/dependabot` and the PR has the `dependencies` label.
2. The PR is open and is not a draft.
3. The PR does not already have both the `lgtm` and `approved` labels.
4. The PR is not in conflict with the base branch (`mergeable` is not `CONFLICTING`).
5. CI is green. Every check, except the ones listed below, has completed with `SUCCESS`,
   `NEUTRAL`, or `SKIPPED`.

Ignore these checks when you evaluate CI, because their state does not reflect the PR's health:

| Check        | Why it is ignored                                                  |
| ------------ | ------------------------------------------------------------------ |
| `tide`       | Stays `PENDING` until the PR has the `lgtm` and `approved` labels. |
| `ok-to-test` | The "Approve Workflow Runs" job is often `CANCELLED` by design.    |

Do NOT approve the PR if any other check is:

- `FAILURE`, `TIMED_OUT`, `ACTION_REQUIRED`, `STARTUP_FAILURE`, or `ERROR`.
- `CANCELLED`, for example a cancelled GPU E2E test. A cancelled check gives no signal, so treat
  it as not green.
- Still running (`PENDING`, `QUEUED`, `IN_PROGRESS`, `WAITING`, or `EXPECTED`).

Never rerun failed checks, push commits, or comment on PRs you skip, unless the user asks.

## Workflow

### 1. List the candidate PRs

```bash
gh pr list -R kubeflow/trainer --author app/dependabot --label dependencies --state open \
  --limit 100 --json number,title,isDraft
```

### 2. Evaluate each PR

For each PR, fetch the labels, mergeable state, and the checks that block approval:

```bash
gh pr view <number> -R kubeflow/trainer --json number,title,isDraft,mergeable,labels,statusCheckRollup -q '
  {
    number, title, isDraft, mergeable,
    labels: [.labels[].name],
    blocking: [
      .statusCheckRollup[]
      | {
          name: (.name // .context),
          result: (if .__typename == "CheckRun"
                   then (if .status == "COMPLETED" then .conclusion else .status end)
                   else .state end)
        }
      | select(.name != "tide" and .name != "ok-to-test")
      | select(.result != "SUCCESS" and .result != "NEUTRAL" and .result != "SKIPPED")
    ]
  }'
```

The PR's CI is green only when `blocking` is empty and at least one non-ignored check is present.
The filtered output above does not show successful checks; fetch the full rollup with
`gh pr view <number> -R kubeflow/trainer --json statusCheckRollup` to verify that at least one
check other than `tide` or `ok-to-test` exists. Otherwise, skip the PR and report that CI results
are unavailable. A `CheckRun` reports its result in `conclusion` only after `status` is
`COMPLETED`. A `StatusContext`, such as the Prow `tide` status, reports it in `state`.

If `mergeable` is `UNKNOWN`, GitHub has not computed it yet. Do not treat `UNKNOWN` as a conflict.

### 3. Show the plan before posting

Print a table for the user with two groups:

- **To approve**: the PR number and title.
- **Skipped**: the PR number, title, and the reason, such as the names of the failing checks.

Also call out PRs to approve that bump a **major** version, such as `36.0.3 to 37.0.1`, or
that update the Kubernetes group. These PRs can be approved when CI is green, but the user should
know about them.

### 4. Approve the PRs

Approve each PR with a GitHub Approve review. Put the Prow commands in the review body, on
separate lines. Do NOT post them as a plain PR comment (`gh pr comment`): the review must
both approve the PR on GitHub and trigger Prow.

```bash
gh pr review <number> -R kubeflow/trainer --approve --body $'/lgtm\n/approve'
```

To approve several PRs, run them in one Bash command so the user confirms the batch once:

```bash
for n in <number> <number> <number>; do
  gh pr review "$n" -R kubeflow/trainer --approve --body $'/lgtm\n/approve' \
    && echo "approved $n" || echo "FAILED $n"
done
```

### 5. Verify the labels

Prow needs up to a minute to process the reviews. Then confirm that it has added both labels:

```bash
gh pr view <number> -R kubeflow/trainer --json state,labels \
  -q '"\(.state) \([.labels[].name | select(. == "lgtm" or . == "approved")] | join(","))"'
```

If a label is missing, check the PR's latest comments from `google-oss-prow` for the reason, for
example that the user is not listed in `OWNERS`, and report it.

### 6. Report the results

Summarize for the user:

- The PRs you approved, and whether Prow added the `lgtm` and `approved` labels.
- The PRs you skipped, with the failing or cancelled checks for each one.
