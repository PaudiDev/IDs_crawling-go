#!/bin/bash

# Must be run from the root of the project (./scripts/run.sh)

docker run --rm -it -p 6060:6060 --env-file .env -v $(pwd)/log:/usr/src/crawler/log crawler