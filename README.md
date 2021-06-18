# GML - GPU memory limiter
![build](https://github.com/tna0y/gml/actions/workflows/go.yml/badge.svg)
[![GitHub release](https://img.shields.io/github/release/tna0y/gml.svg)](https://GitHub.com/tna0y/gml/releases/)

Impose a GPU memory limit on any process.

Utility was originally designed to provide a workaround for absence of GPU memory limit in `nvidia-docker` but works for any
process on Linux.

## How it works

GML runs any command and monitors its GPU memory usage. If the limit is exceded GML sends a signal (default: SIGKILL) to
the process and waits until it returns.

## Install
Pre-compiled binary is available for Linux / x86_64. Install it with following commands:
```shell script
curl -o gml https://github.com/tna0y/gml/releases/download/v0.1/gml-linux-x86_64
sudo cp gml /usr/local/bin/gml
```

## Usage

```
Usage: gml [--limit LIMIT] [--signal SIGNUM] -- COMMAND
      --help            show help message
  -l, --limit string    total GPU memory limit for the command (default "1MB")
  -s, --signal string   signal that will be sent to the process if the memory exceeds the limit (default "SIGKILL")
```

### Example

```shell script
gml --limit 1GB --signal SIGINT -- ./my-gpu-app arg1 arg2
```
