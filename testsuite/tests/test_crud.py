import pytest
import requests
import json


@pytest.fixture
def created_note(base_url: str, api_session: requests.Session, login_token: str):
    note_data = {
        "title": "Test Note",
        "content": "This is a test note content"
    }

    response = api_session.post(
        f"{base_url}/notes",
        headers={"Authorization": f"{login_token}"},
        data=note_data
    )

    assert response.status_code == 200
    json_response = response.json()

    assert "data" in json_response
    note = json_response["data"]
    assert "id" in note
    assert note["title"] == note_data["title"]
    assert note["content"] == note_data["content"]
    return note


def test_create_note_success(created_note):
    assert created_note["id"] is not None
    assert created_note["title"] == "Test Note"
    assert created_note["content"] == "This is a test note content"


def test_get_note_by_id(base_url: str, api_session: requests.Session, created_note, login_token: str):
    note_id = created_note["id"]

    response = api_session.get(
        f"{base_url}/notes/{note_id}",
        headers={"Authorization": f"{login_token}"}
    )

    assert response.status_code == 200
    json_response = response.json()

    assert "data" in json_response
    note = json_response["data"]
    assert note["id"] == note_id
    assert note["title"] == created_note["title"]
    assert note["content"] == created_note["content"]

def test_update_note(base_url: str, api_session: requests.Session, created_note, login_token: str):
    note_id = created_note["id"]

    response = api_session.put(
        f"{base_url}/notes/{note_id}",
        headers={"Authorization": f"{login_token}"},
        data={
            "title": "Updated Test Note",
            "content": "Updated Test Note Content"
        }
    )

    assert response.status_code == 200
    json_response = response.json()
    assert json_response["status"] == "success"

    updated_get_note = api_session.get(
        f"{base_url}/notes/{note_id}",
        headers={"Authorization": f"{login_token}"}
    )
    assert updated_get_note.status_code == 200

    json_response = updated_get_note.json()

    assert "data" in json_response
    note = json_response["data"]
    assert note["id"] == note_id
    assert note["title"] == "Updated Test Note"
    assert note["content"] == "Updated Test Note Content"

def test_get_note_invalid_id(base_url: str, api_session: requests.Session, login_token: str):
    invalid_id = "999999"  # Assuming this ID doesn't exist

    response = api_session.get(
        f"{base_url}/notes/{invalid_id}",
        headers={"Authorization": f"{login_token}"}
    )

    assert response.status_code == 404


def test_create_note_missing_params(base_url: str, api_session: requests.Session, login_token: str):
    incomplete_data = {
        "title": "Incomplete Note"
    }

    response = api_session.post(
        f"{base_url}/notes",
        headers={"Authorization": f"{login_token}"},
        data=incomplete_data
    )

    assert response.status_code == 400


def test_delete_note(base_url: str, api_session: requests.Session, login_token: str):
    # Create a new note specifically for deletion test
    note_data = {
        "title": "Note To Delete",
        "content": "This note will be deleted"
    }

    # Create the note
    create_response = api_session.post(
        f"{base_url}/notes",
        headers={"Authorization": f"{login_token}"},
        data=note_data
    )
    assert create_response.status_code == 200

    note_id = create_response.json()["data"]["id"]

    # Delete the note
    delete_response = api_session.delete(
        f"{base_url}/notes/{note_id}",
        headers={"Authorization": f"{login_token}"}
    )
    assert delete_response.status_code == 200
    assert delete_response.json()["status"] == "success"

    # Try to get the deleted note - should return 404
    get_response = api_session.get(
        f"{base_url}/notes/{note_id}",
        headers={"Authorization": f"{login_token}"}
    )
    assert get_response.status_code == 404


def test_list_notes(base_url: str, api_session: requests.Session, login_token: str):
    """Test listing all notes for the authenticated user."""
    # Create multiple notes for testing the list functionality
    note_data = [
        {
            "title": "First List Test Note",
            "content": "First note content for list test"
        },
        {
            "title": "Second List Test Note",
            "content": "Second note content for list test"
        },
        {
            "title": "Third List Test Note",
            "content": "Third note content for list test"
        }
    ]

    created_notes = []

    # Create the test notes
    for data in note_data:
        response = api_session.post(
            f"{base_url}/notes",
            headers={"Authorization": f"{login_token}"},
            data=data
        )
        assert response.status_code == 200
        created_notes.append(response.json()["data"])

    # Get the list of notes
    list_response = api_session.get(
        f"{base_url}/notes",
        headers={"Authorization": f"{login_token}"}
    )

    # Verify the response
    assert list_response.status_code == 200
    json_response = list_response.json()

    assert "status" in json_response
    assert json_response["status"] == "success"
    assert "data" in json_response

    # Get the notes from the response
    notes_list = json_response["data"]

    # Verify all created notes are in the list
    for created_note in created_notes:
        found = False
        for note in notes_list:
            if note["id"] == created_note["id"]:
                found = True
                assert note["title"] == created_note["title"]
                assert note["content"] == created_note["content"]
                break
        assert found, f"Note with ID {created_note['id']} not found in the list response"

    # Clean up - delete all created notes
    for note in created_notes:
        delete_response = api_session.delete(
            f"{base_url}/notes/{note['id']}",
            headers={"Authorization": f"{login_token}"}
        )
        assert delete_response.status_code == 200
