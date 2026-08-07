---
name: Feature request
about: A new resource, data source, or attribute
title: "[FEATURE] "
labels: [enhancement]
assignees: []
---

## What are you trying to build?

<!-- One-paragraph description of the use case — what Terraform workflow
does this feature enable? -->

## What's missing today?

<!-- What resource / data source / attribute would unlock the use case?
Include the Cloud API endpoint(s) that back it if you know them. -->

## Proposed HCL

<!-- Show what your HCL would look like once the feature ships. -->

```hcl
resource "laravelcloud_<new_resource>" "example" {
  # your proposal here
}
```

## Cloud API endpoints

<!-- If you know the Cloud API endpoints that back the feature, list
them here. Otherwise leave "unknown" and the maintainer will map it. -->

- `GET /…`
- `POST /…`
- `PATCH /…`
- `DELETE /…`

## Workarounds

<!-- Are you working around the gap today? Describe how — this helps
the maintainer estimate the impact of the missing feature. -->

## Additional context

<!-- Anything else that helps prioritise — is this blocking a production
migration? A team-wide rollout? -->
