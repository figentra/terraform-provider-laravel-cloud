# Summary

<!-- One-paragraph description of the change — what does this PR do?
Reference the issue you're closing (`Closes #123`). -->

## Change kind

<!-- Check the box that applies. -->

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would change existing behaviour)
- [ ] Docs-only (no code change)
- [ ] Test-only (no code change)
- [ ] Refactor (no behavioural change)
- [ ] CI / tooling

## Checklist

- [ ] `make fmt` — Go + Terraform examples formatted.
- [ ] `make lint` — golangci-lint clean.
- [ ] `make test` — unit tests pass.
- [ ] `make testacc` — acceptance tests pass against a real Cloud tenant.
      **OR** — the change is documentation / CI / examples-only.
- [ ] `make docs` — docs/ regenerated (if a schema changed).
- [ ] `CHANGELOG.md` updated with the delta.
- [ ] New resources have matching examples + data source + acceptance
      test (see `CONTRIBUTING.md` §"Adding a new resource").

## Terraform output

<!-- If this PR adds or changes a resource, paste the `terraform plan`
output showing the new attribute rendering (or a `terraform apply` if
you can). REDACT any confidential IDs. -->

```
paste plan here
```

## Related links

<!-- Cloud API changelog / dashboard screenshots / adjacent issues / etc. -->
