---
name: Bug report
about: A resource misbehaves against the Cloud API or the provider crashes
title: "[BUG] "
labels: [bug]
assignees: []
---

## What happened?

<!-- One-paragraph description of the misbehaviour. Include the resource
type + attribute involved. -->

## What did you expect?

<!-- One-paragraph description of the intended behaviour. If you're
referencing a Cloud dashboard behaviour, link the dashboard URL. -->

## Terraform + provider versions

```
$ terraform version
Terraform vX.Y.Z
+ provider registry.terraform.io/figentra/laravel-cloud vA.B.C
```

## Minimal reproduction

Paste the smallest `.tf` file that reproduces the bug:

```hcl
terraform {
  required_providers {
    laravelcloud = {
      source  = "figentra/laravel-cloud"
      version = "~> 0.3"
    }
  }
}

# your minimal repro here
```

## Terraform log

<!-- Run with `TF_LOG=DEBUG terraform apply 2>&1 | tee tf.log` and paste
relevant excerpts. REDACT tokens, IDs, and any confidential resource
names before pasting. -->

<details>
<summary>Click to expand log</summary>

```
paste log here
```

</details>

## Cloud API response

<!-- If you have a raw Cloud API response that reproduces the bug (e.g.
the shape doesn't match the provider's model), paste it here. REDACT
tokens + confidential fields. -->

## Additional context

<!-- Anything else that helps triage — dev vs. prod, region, workspace
context, etc. -->
