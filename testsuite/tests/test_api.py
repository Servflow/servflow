import pytest
import requests
import re


def test_health_endpoint(base_url: str, api_session: requests.Session):
    """Test the health endpoint returns 200."""
    response = api_session.get(f"{base_url}/health")
    assert response.status_code == 200


def test_register_success(base_url: str, api_session: requests.Session, test_user: dict):
    """Test successful user registration."""
    response = api_session.post(
        f"{base_url}/register",
        data=test_user
    )
    print(response.text)
    assert response.status_code == 200
    json_response = response.json()
    assert json_response.get('success') is True


def test_register_duplicate_user(base_url: str, api_session: requests.Session, test_user: dict):
    """Test that registering a duplicate user fails appropriately."""
    response = api_session.post(
        f"{base_url}/register",
        data=test_user
    )
    assert response.status_code != 200, "Duplicate registration should not succeed"


def test_register_invalid_data(base_url: str, api_session: requests.Session):
    """Test registration with invalid data."""
    invalid_user = {
        "name": "",  # Empty name
        "password": "test",
        "email": "invalid-email"  # Invalid email format
    }
    response = api_session.post(
        f"{base_url}/register",
        data=invalid_user
    )
    assert response.status_code != 200, "Registration with invalid data should not succeed"


@pytest.mark.parametrize("invalid_field", [
    {"name": None},
    {"password": None},
    {"email": None}
])
def test_register_missing_required_fields(
    base_url: str,
    api_session: requests.Session,
    test_user: dict,
    invalid_field: dict
):
    """Test registration with missing required fields."""
    invalid_data = test_user.copy()
    invalid_data.update(invalid_field)
    
    response = api_session.post(
        f"{base_url}/register",
        data={k: v for k, v in invalid_data.items() if v is not None}
    )
    assert response.status_code != 200, f"Registration missing {list(invalid_field.keys())[0]} should not succeed"


def test_login_success(base_url: str, api_session: requests.Session, test_user: dict):
    """Test successful login returns a valid JWT token."""
    # First register a user
    api_session.post(
        f"{base_url}/register",
        data=test_user
    )


    # Then try to login
    login_data = {
        "email": test_user["email"],
        "password": test_user["password"]
    }
    response = api_session.post(
        f"{base_url}/login",
        data=login_data
    )
    print(response.text)
    assert response.status_code == 200
    json_response = response.json()
    
    # Verify token exists and has correct JWT format
    assert "token" in json_response
    token = json_response["token"]
    jwt_pattern = r'^[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.[A-Za-z0-9-_.+/=]+$'
    assert re.match(jwt_pattern, token), "Invalid JWT token format"


def test_login_invalid_credentials(base_url: str, api_session: requests.Session):
    """Test login with invalid credentials fails."""
    login_data = {
        "email": "nonexistent@test.com",
        "password": "wrongpassword"
    }
    response = api_session.post(
        f"{base_url}/login",
        data=login_data
    )
    assert response.status_code != 200, "Login with invalid credentials should not succeed"


def test_login_missing_fields(base_url: str, api_session: requests.Session):
    """Test login with missing required fields fails."""
    # Test missing email
    response = api_session.post(
        f"{base_url}/login",
        data={"password": "test"}
    )
    assert response.status_code != 200, "Login without email should not succeed"

    # Test missing password
    response = api_session.post(
        f"{base_url}/login",
        data={"email": "test@test.com"}
    )
    assert response.status_code != 200, "Login without password should not succeed"


def test_login_empty_fields(base_url: str, api_session: requests.Session):
    """Test login with empty fields fails."""
    login_data = {
        "email": "",
        "password": ""
    }
    response = api_session.post(
        f"{base_url}/login",
        data=login_data
    )
    assert response.status_code != 200, "Login with empty fields should not succeed"