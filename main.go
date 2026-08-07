// Package main is the Terraform Provider for Laravel Cloud entry point.
//
// Publishes as `registry.terraform.io/figentra/laravel-cloud`. Every consumer
// declares this in a `required_providers` block:
//
//	terraform {
//	  required_providers {
//	    laravelcloud = {
//	      source  = "figentra/laravel-cloud"
//	      version = "~> 0.1"
//	    }
//	  }
//	}
//
// See:
//   - ADR-0080 (.docs/adr/0080-terraform-for-cloud-devops.md) — the pivot decision
//   - Migration plan (.kiro/plans/2026-08-07-terraform-pivot-plan.md) — 5-phase playbook
//   - Agent charter (.kiro/agents/go-terraform-provider-builder.md) — owning agent
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/provider"
)

// version is stamped by GoReleaser via `-ldflags "-X main.version=<tag>"`
// at release time. Development builds ship the string below unchanged so
// operators can tell a `make install` build from a Registry release.
var version = "dev"

// main boots the provider gRPC server that Terraform Core talks to over
// stdin/stdout during a `terraform plan` / `apply` cycle. The `-debug`
// flag is HashiCorp-canonical: it prints a `TF_REATTACH_PROVIDERS` env
// var operators paste into their shell for interactive debugging via
// dlv / VSCode / etc.
func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// Registry address MUST match the tf-registry publish target.
		// `figentra/laravel-cloud` in the public registry.
		Address: "registry.terraform.io/figentra/laravel-cloud",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
