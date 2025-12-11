#!/bin/bash

set -ex

# Build the Go binary
pushd ..
GOOS=linux go build -o servflow || true
#./servflow validate testsuite/compose/confs
popd

# Install Python dependencies if not already installed
if [ ! -d "venv" ]; then
    python3 -m venv venv
    . venv/bin/activate
    pip install -r requirements.txt
else
    . venv/bin/activate
fi

# Navigate to the directory containing the docker-compose file
pushd ./compose || exit

# Build the images and start the services defined in the docker-compose file
docker compose up --build --force-recreate -d

popd

# Run pytest with the Docker services running
pytest tests/ -v

# Get the pytest exit code
TEST_EXIT_CODE=$?

# Clean up Docker services
pushd ./compose || exit
docker compose down
docker compose rm -f
popd

# Deactivate virtual environment
deactivate

# Exit with the pytest exit code
exit $TEST_EXIT_CODE
