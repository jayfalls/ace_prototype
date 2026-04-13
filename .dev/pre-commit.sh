#!/bin/bash
set -e
cd "$(git rev-parse --show-toplevel)"
make test
