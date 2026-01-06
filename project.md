# Interview Project

## Task 1 -- Build & Ship a Multi-Arch Container Image (Lucene Shard Analyzer Service)

Create a small, self-contained service called **Lucene Shard Analyzer Service**, then set up a GitHub Actions CI/CD pipeline that builds and pushes Docker images for it.

### Service Spec

A stateless HTTP service that:

- Listens on port `8080`
- Endpoints:
  - `GET /healthz` -> returns `200 OK` with body `ok`
  - `GET /info` -> returns JSON including at least:
    - `version` (string)
    - `git_sha` (string)
    - `arch` (e.g., amd64/arm64)
    - `hostname`
  - `GET /metrics` -> Prometheus-style metrics
  - `POST /analyze` -> upload an OpenSearch/Elasticsearch shard archive (tar/zip) and analyze Lucene segments offline

### `POST /analyze`

#### Archive assumptions
- The archive represents a **single shard**.
- The archive must contain **exactly one** Lucene index directory.

#### Response
Return a **JSON report** of your analysis.

- The goal is to expose **as much useful information as possible** from the Lucene index and its segments.
- You are free to design the JSON structure, but it should be reasonably structured (e.g., top-level summary + optional per-segment details).

At minimum, the report should include:
- Total number of segments in the shard
- Total document count and total deleted document count (as represented in the segments)
- Per-segment information, including at least:
  - segment identifier/name
  - docs count
  - deleted docs count
  - any other interesting details you want to add

### CI/CD Requirements
Set up GitHub Actions to:

- Build **multi-architecture** Docker images for:
  - `amd64`
  - `arm64`
- Push images to a container registry (recommended: **GitHub Container Registry / GHCR**)
- Use a sensible tagging strategy, e.g.:
  - `latest` (main branch)
  - `vX.Y.Z` (on tags)
  - `sha-<shortsha>` (for traceability)

---

## Task 2 -- Kubernetes Deployment + Automated Integration Test

Set up a GitHub Actions workflow that deploys **Lucene Shard Analyzer Service** into a Kubernetes cluster and validates it with an automated test.

### Requirements

1. **Deploy multiple instances in the cluster**
   - You may choose any reasonable Kubernetes workload type (e.g., Deployment, StatefulSet, etc.)
   - Ensure there are **multiple Pods** running (e.g., replicas >= 2)

2. **Expose the service and ensure even traffic distribution**
   - Expose the service so it can be accessed via a stable endpoint (inside or outside the cluster)
   - You may choose any reasonable approach (e.g., Service + Ingress, Service + Gateway API, Service + LoadBalancer, NodePort, etc.)
   - **Traffic must be evenly distributed across the multiple Pods** (demonstrate this in your test or verification steps)

3. **Add an automated test case**
   The pipeline should run a test that uses the deployed service, for example:
   - Call `GET /healthz` and expect success
   - Call `GET /info` multiple times and show that responses come from different pods/hostnames (to demonstrate load balancing)
   - Call `POST /analyze` using a sample shard archive and verify the service returns a successful JSON report

---

## Task 3 -- Safer PR Testing with a Shared Staging Cluster (Prototype)

Assuming that we already have:

- One shared Kubernetes staging environment
- Multiple services/components in a single monorepo
- We'd like to improve how we test pull requests (PRs) before merging

Today, if we deploy a PR directly to staging, it can break other people's work who are also using staging. We want a better approach.

### Goal
Design and prototype a solution where:

- For each PR, we can deploy that PR's version of a service (or services) to the staging cluster
- We can run tests against the PR version
- The normal staging environment continues to work for everyone else (PR deployments should not break shared staging)

You are free to choose any reasonable mechanisms to achieve this.

We're mainly interested in:

- Your design and reasoning
- How you would wire this into CI

It's okay to make reasonable assumptions and leave some parts as "future work" -- just explain them clearly. Partial solutions are okay -- just make your assumptions and limitations explicit.

---

## We DO NOT expect

- A fully deployed production system
- A perfectly generic solution for all environments
- Deep expertise in any specific tool
