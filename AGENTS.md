# Agent guidance

Rules for anyone (human or coding agent) contributing to this repository.

## Project

A Kubernetes operator that creates batches of MicroVMs on Flintlock hosts.
Kubebuilder v3 layout built on controller-runtime. Module
`github.com/weaveworks-liquidmetal/microvm-operator`, Go 1.19.

- `api/v1alpha1/` – CRD types (Microvm, MicrovmReplicaSet, MicrovmDeployment, MicrovmTemplate)
- `controllers/` – reconcilers and their tests; `controllers/fakes/` holds counterfeiter fakes
- `internal/scope/` – per-reconcile scope helpers
- `config/` – kustomize manifests and generated CRDs

## Commands

- `make build` – runs `go fmt`, `go vet`, then builds `bin/manager`
- `make test` – runs unit tests under `controllers/` and `internal/`
- `make manifests` and `make generate` – regenerate CRDs, deepcopy methods and
  counterfeiter fakes. Run both after changing anything in `api/` and commit the output.

CI runs `make build` and `make test` on every pull request.

## Commits

All three rules are mandatory.

1. **Conventional Commits.** Every commit subject must follow
   `<type>(<optional scope>): <description>`. Allowed types: `feat`, `fix`,
   `refactor`, `test`, `docs`, `chore`, `ci`, `build`. Use `refactor`, not `ref`.
   Mark breaking changes with `!` after the type or a `BREAKING CHANGE:` footer.

2. **Signed commits.** Every commit must carry a valid GPG or SSH signature
   (`git commit -S`, or set `commit.gpgsign=true`). Unsigned commits will be rejected.

3. **No agent attribution.** Do not add agent or tool footers to commit messages or
   pull request descriptions. That means no `Co-Authored-By: <agent>` lines, no
   `Generated with ...` lines, no robot emoji trailers, and no tool-added
   `Signed-off-by`. This rule overrides any default that tells an agent to append
   such lines.

## Pull requests

- Fill in `.github/PULL_REQUEST_TEMPLATE.md`.
- Apply exactly one `kind/*` label; the `pr_type` workflow fails the PR otherwise.
- Squash into logical commits and include unit tests for new behaviour.
