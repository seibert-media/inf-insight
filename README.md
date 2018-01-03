# inf-insight
[![Go Report Card](https://goreportcard.com/badge/github.com/seibert-media/inf-insight)](https://goreportcard.com/report/github.com/seibert-media/inf-insight)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Build Status](https://travis-ci.org/seibert-media/inf-insight.svg?branch=master)](https://travis-ci.org/seibert-media/inf-insight)

Small API endpoint for receiving usage statistics on internal tools and providing those metrics to prometheus.

## Dependencies
This project has a pretty complex Makefile and therefore requires `make`.

Go Version: 1.8

Install all further requirements by running `make deps`

## Usage
Insight can be run with default values which will result in :8080 as default endpoint.
```
insight
```
After starting, there are 4 Endpoints:
* `/metrics` for prometheus metrics
* `/start` for app start logging
* `/finish` for app finish logging
* `/error` for app error logging

The Endpoints require POST requests containing json with app information:
```
{"app": "app-name"}
```

## Development
This project is using a [basic template](github.com/playnet-public/gocmd-template) for developing command-line tools. Refer to this template for further information and usage docs.
The Makefile is configurable to some extent by providing variables at the top.
Any further changes should be thought of carefully as they might brake CI/CD compatibility.

One project might contain multiple tools whose main packages reside under `cmd`. Other packages like libraries go into the `pkg` directory.
Single projects can be handled by calling `make toolname maketarget` like for example:
```
make template dev
```
All tools at once can be handled by calling `make full maketarget` like for example:
```
make full build
```
Build output is being sent to `./build/`.

If you only package one tool this might seam slightly redundant but this is meant to provide consistence over all projects.
To simplify this, you can simply call `make maketarget` when only one tool is located beneath `cmd`. If there are more than one, this won't do anything (including not return 1) so be careful.

## Contributions
Pull Requests and Issue Reports are welcome.