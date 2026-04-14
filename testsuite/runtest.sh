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
# Temporarily disable exit-on-error to capture exit code
set +e
pytest tests/ -v
TEST_EXIT_CODE=$?
set -e

# Show servflow container logs if tests failed
if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo "=== Tests failed. Showing servflow container logs ==="
    pushd ./compose || exit
    docker compose logs servflow
    popd
    echo "=== End of servflow logs ==="
fi

# Clean up Docker services
pushd ./compose || exit
docker compose down
docker compose rm -f
popd

# Deactivate virtual environment
deactivate

# Exit with the pytest exit code
exit $TEST_EXIT_CODE
