# krepl — An interactive REPL for Kubernetes

krepl is an interactive REPL for managing Kubernetes clusters and is inspired by [Click](https://github.com/databricks/click). One of its goals is best-effort Click compatibility, so Click users can also use krepl in an intuitive way. It maintains context, namespace, and selected object state between commands, so you do not need to pass `--context` and `--namespace` flags or object names on every invocation. It also supports a few capabilities Click does not currently support, including editing Kubernetes objects and running without a dependency on the `kubectl` binary.

## Code Status
![build and test](https://github.com/heyihong/krepl/actions/workflows/build_and_test.yml/badge.svg?branch=main)

## Requirements

- Go 1.25+
- A valid kubeconfig (`~/.kube/config` or `$KUBECONFIG`)

## Installation

Install directly with `go install`:

```sh
go install github.com/heyihong/krepl/cmd/krepl@latest
```

Or, from a local clone:

```sh
git clone https://github.com/heyihong/krepl.git
cd krepl
go install ./cmd/krepl
```

## Getting Started

If you installed the binary with `go install`, run:

```sh
krepl
```

If you cloned the repository and want to run without installing, use the startup script:

```sh
bin/krepl.sh
```

For command discovery, type `help` in the repl. For detailed command help, use `help <command>` or `<command> --help`.

## Demo Screenshot

<img width="715" height="390" alt="Screenshot 2026-04-09 at 9 31 34 PM" src="https://github.com/user-attachments/assets/44359018-9f52-4cc2-9d27-446e9a73f287" />
