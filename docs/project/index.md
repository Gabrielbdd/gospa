# Project Documentation

This directory contains contributor and maintainer documentation for
Gospa. It covers internal processes and conventions — not product
design. For the product blueprint (market, rules, features, roadmap),
see [`../blueprint/index.md`](../blueprint/index.md).

## Contents

- [Agent Workflow](agent-workflow.md) — Progress file convention for
  long-running work that spans more than one session.

## What belongs here

- Contributor workflows (progress files, review conventions, release
  procedures when they exist).
- Internal migration notes and upgrade plans.
- Anything that helps someone contribute to Gospa but does not belong
  in the product blueprint.

## What does not belong here

- Product rules, features, and roadmap — those live in
  `docs/blueprint/`.
- API reference — the authoritative API surface is the proto set under
  `proto/gospa/**`.
- Framework concerns — those belong upstream in the Gofra repository.
