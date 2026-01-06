# PR Testing Design for Shared Kubernetes Staging Cluster

## Overview
This document describes the design for safely testing pull requests (PRs) in a shared Kubernetes staging cluster without disrupting the normal staging environment.

## Problem
Currently, deploying a PR directly to the staging cluster can break other people's work who are also using the staging environment. We need a way to deploy PR versions of services in isolation while still being able to run tests against them.

## Solution
We will implement a solution that uses **namespace isolation** and **traffic routing** to allow multiple versions of services to run in the same cluster without interfering with each other.

### Key Components

1. **Namespace Isolation**: Each PR will be deployed to its own Kubernetes namespace.
2. **Dynamic Environment Variables**: PR-specific configuration will be injected into deployments.
3. **Ingress with Path-Based Routing**: Traffic will be routed to the appropriate PR namespace based on path prefix.
4. **GitHub Actions Workflow**: Automate the deployment and testing process.

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     Shared Cluster                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────┐
│  │ PR-123 Namespace│    │ PR-456 Namespace│    │ Staging │
│  │                 │    │                 │    │ Namespace│
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────┐ │
│  │ │ Service A   │ │    │ │ Service A   │ │    │ │ A   │ │
│  │ └─────────────┘ │    │ └─────────────┘ │    │ └─────┘ │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────┐ │
│  │ │ Service B   │ │    │ │ Service B   │ │    │ │ B   │ │
│  │ └─────────────┘ │    │ └─────────────┘ │    │ └─────┘ │
│  └─────────────────┘    └─────────────────┘    └─────────┘
│                                                         │
│  ┌─────────────────────────────────────────────────────┐ │
│  │                      Ingress                         │ │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────┐   │ │
│  │  │ /pr/123/*   │    │ /pr/456/*   │    │ /*      │   │ │
│  │  └─────────────┘    └─────────────┘    └─────────┘   │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. Namespace Creation
- Each PR will get its own namespace with the format `pr-{pr-number}`.
- The namespace will be created when the PR is opened and deleted when it's merged or closed.

### 2. Deployment Process
1. **Build and Push**: Build the Docker image for the PR with a unique tag (e.g., `pr-123`).
2. **Create Namespace**: Create a new namespace for the PR.
3. **Deploy Services**: Deploy the services to the PR namespace using the PR-specific image tag.
4. **Configure Ingress**: Update the ingress to route traffic from `/pr/{pr-number}/*` to the PR namespace.
5. **Run Tests**: Run automated tests against the PR deployment.

### 3. GitHub Actions Workflow

```yaml
name: PR Testing in Shared Cluster

on:
  pull_request:
    types: [opened, synchronize, closed]

jobs:
  pr-testing:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up kubectl
        uses: azure/setup-kubectl@v4

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        if: github.event.action != 'closed'
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: ghcr.io/${{ github.repository }}:pr-${{ github.event.pull_request.number }}

      - name: Create PR namespace
        if: github.event.action != 'closed'
        run: |
          kubectl create namespace pr-${{ github.event.pull_request.number }} --dry-run=client -o yaml | kubectl apply -f -

      - name: Deploy to PR namespace
        if: github.event.action != 'closed'
        run: |
          # Use kustomize to create PR-specific manifests
          kustomize build k8s/overlays/pr --load-vars-from=pr-vars.yaml | kubectl apply -f -

      - name: Update ingress for PR
        if: github.event.action != 'closed'
        run: |
          # Add PR-specific ingress rules
          kubectl apply -f - <<EOF
          apiVersion: networking.k8s.io/v1
          kind: Ingress
          metadata:
            name: pr-${{ github.event.pull_request.number }}-ingress
          spec:
            rules:
            - host: staging.example.com
              http:
                paths:
                - path: /pr/${{ github.event.pull_request.number }}/
                  pathType: Prefix
                  backend:
                    service:
                      name: lucene-shard-analyzer-service
                      port:
                        number: 80
            EOF

      - name: Run tests against PR deployment
        if: github.event.action != 'closed'
        run: |
          PR_URL="https://staging.example.com/pr/${{ github.event.pull_request.number }}"
          # Run health check
          curl -f $PR_URL/healthz
          # Run info check
          curl -f $PR_URL/info
          # Run analyze test with sample data
          curl -X POST -F "archive=@sample-shard.tar.gz" $PR_URL/analyze

      - name: Clean up PR resources
        if: github.event.action == 'closed'
        run: |
          # Delete PR namespace and all resources in it
          kubectl delete namespace pr-${{ github.event.pull_request.number }} --ignore-not-found
```

### 4. Kustomize Setup
We will use Kustomize to generate PR-specific manifests:

```
k8s/
├── base/
│   ├── deployment.yml
│   ├── service.yml
│   └── kustomization.yml
└── overlays/
    └── pr/
        ├── kustomization.yml
        └── patch-image-tag.yml
```

#### k8s/overlays/pr/kustomization.yml
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
- ../../base

patchesStrategicMerge:
- patch-image-tag.yml

namespace: pr-${PR_NUMBER}
```

#### k8s/overlays/pr/patch-image-tag.yml
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lucene-shard-analyzer
spec:
  template:
    spec:
      containers:
      - name: lucene-shard-analyzer
        image: ghcr.io/${REPOSITORY}:pr-${PR_NUMBER}
```

## Benefits

1. **Isolation**: PRs are deployed in their own namespaces, preventing conflicts with other PRs and the main staging environment.
2. **Cost Efficiency**: No need for separate clusters for each PR.
3. **Automation**: The entire process is automated through GitHub Actions.
4. **Easy Testing**: PRs can be tested using a predictable URL format.
5. **Cleanup**: Resources are automatically cleaned up when PRs are closed or merged.

## Limitations

1. **Resource Constraints**: The shared cluster must have enough resources to handle multiple PR deployments simultaneously.
2. **Cross-Service Dependencies**: If services depend on each other, all dependent services must be deployed to the PR namespace.
3. **Database Isolation**: Additional setup may be required for database isolation if services use databases.

## Future Enhancements

1. **Auto-Scaling PR Namespaces**: Scale PR deployments based on resource usage.
2. **Canary Deployments**: Gradually roll out changes to the main staging environment after PR testing.
3. **Integration with Monitoring**: Add monitoring and alerting for PR deployments.
4. **Self-Service PR Testing**: Allow developers to manually trigger PR tests through GitHub UI.

## Conclusion
This design provides a safe and efficient way to test PRs in a shared Kubernetes staging cluster. By using namespace isolation and path-based routing, we can ensure that PR deployments don't interfere with each other or the main staging environment, while still allowing for comprehensive testing.
