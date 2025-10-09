import pytest
import requests
import time
from typing import Generator
from datetime import datetime


@pytest.fixture(scope="session")
def base_url() -> str:
    """Fixture to provide the base URL for the API."""
    return "http://localhost:8181"


@pytest.fixture(scope="session")
def api_health_check(base_url: str) -> None:
    """Fixture to ensure API is healthy before running tests."""
    start_time = time.time()
    timeout = 30  # seconds

    while True:
        try:
            response = requests.get(f"{base_url}/health")
            if response.status_code == 200:
                return
        except requests.RequestException:
            pass

        if time.time() - start_time > timeout:
            raise TimeoutError("API health check failed after 30 seconds")

        time.sleep(2)


@pytest.fixture(scope="session")
def test_user() -> dict:
    """Fixture providing test user data with timestamp-based unique identifiers."""
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    return {
        "name": f"test_user_{timestamp}",
        "password": "test",
        "email": f"test_{timestamp}@test.com"
    }


@pytest.fixture(scope="session")
def api_session(base_url: str, api_health_check) -> Generator[requests.Session, None, None]:
    """Fixture providing a session for making API requests."""
    session = requests.Session()
    yield session
    session.close()


@pytest.fixture(scope="session")
def login_token(base_url: str, api_session: requests.Session, test_user: dict) -> str:
    """Fixture to get a valid login token for the test user."""
    # First ensure the user is registered
    try:
        api_session.post(
            f"{base_url}/register",
            data=test_user
        )
    except requests.RequestException:
        # User might already be registered, which is fine
        pass

    # Then login to get the token
    login_data = {
        "email": test_user["email"],
        "password": test_user["password"]
    }
    response = api_session.post(
        f"{base_url}/login",
        data=login_data
    )
    assert response.status_code == 200, "Failed to login and get token"
    json_response = response.json()

    # Verify token exists
    assert "token" in json_response, "No token found in login response"
    return json_response["token"]
