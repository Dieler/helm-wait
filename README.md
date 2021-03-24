# Helm Wait Plugin
[![Go Report Card](https://goreportcard.com/badge/github.com/dieler/helm-wait)](https://goreportcard.com/report/github.com/dieler/helm-wait)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/dieler/helm-wait/blob/master/LICENSE)

This is a Helm plugin allowing to introduce wait conditions, e.g. in CI/CD pipelines before running integration tests,
checking if all changes of a Helm install/ugrade step have been applied.
It differs from the Helm wait option in that it checks if all pods of a stateful set or deployment have been replaced and are up and running.

*The implementation of this plugin is inspired by the [Helm Diff plugin](https://github.com/databus23/helm-diff)
and uses large portions of it for computing the diff between revisions of releases,
so a lot of credits go to the contributors of that repository.*

## Usage

```
Usage:
  wait [command]

Available Commands:
  upgrade     Wait until all the changes of the current release have been applied
```

## Commands:

### upgrade:

```shell
$ helm wait upgrade -h
This command compares the current revision of the given release with its previous revision
and waits until all changes of the current revision have been applied.

Usage:
  wait upgrade [RELEASE]

Examples:
  helm wait upgrade my-release
  helm wait upgrade my-release --timeout 600
```

## Install

Based on the version in plugin.yaml, release binary will be downloaded from GitHub:
```shell
$ helm plugin install https://github.com/Dieler/helm-wait.git
```

## Build

```shell
$ git clone https://github.com/dieler/helm-wait.git
$ cd helm-wait
$ make install
```

The above will install this plugin into your `$HELM_PLUGINS` directory.

### Prerequisites

- You need to have [Go 1.16](http://golang.org) installed.

