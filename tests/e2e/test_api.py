"""End-to-end tests for tokentracer-proxy API.

Requires the full docker-compose stack (postgres + redis + app) to be running.
Run with: pytest -v test_api.py
"""

import os
import uuid

import pytest
import requests


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def base_url():
    return os.environ.get("TEST_BASE_URL", "http://localhost:8080")


@pytest.fixture
def unique_user(base_url: str):
    """Create a fresh user, log in, and return credentials + auth headers."""
    uid = uuid.uuid4().hex[:12]
    email = f"test-{uid}@e2e.local"
    password = "TestPass123!"

    # Signup
    r = requests.post(
        f"{base_url}/auth/signup",
        json={"email": email, "password": password},
    )
    assert r.status_code == 201, f"Signup failed: {r.status_code} {r.text}"

    # Login
    r = requests.post(
        f"{base_url}/auth/login",
        json={"email": email, "password": password},
    )
    assert r.status_code == 200, f"Login failed: {r.status_code} {r.text}"
    token = r.json()["token"]
    
    # TODO: type
    return {
        "email": email,
        "password": password,
        "token": token,
        "headers": {"Authorization": f"Bearer {token}"},
    }


@pytest.fixture
def provider_key(base_url: str, unique_user):
    """Create a provider key and return the provider list."""
    r = requests.post(
        f"{base_url}/manage/providers",
        headers=unique_user["headers"],
        json={
            "provider": "openai",
            "api_key": "sk-fake-test-key-" + uuid.uuid4().hex[:8],
            "label": "E2E Test Key",
        },
    )
    assert r.status_code == 201, f"Create provider key failed: {r.status_code} {r.text}"

    r = requests.get(
        f"{base_url}/manage/providers",
        headers=unique_user["headers"],
    )
    assert r.status_code == 200
    providers = r.json()
    assert len(providers) >= 1
    return providers


# ---------------------------------------------------------------------------
# 1. Auth flow
# ---------------------------------------------------------------------------


class TestAuthFlow:
    def test_signup_login_me_apikey(self, base_url, unique_user):
        headers = unique_user["headers"]

        # /auth/me
        r = requests.get(f"{base_url}/auth/me", headers=headers)
        assert r.status_code == 200
        assert r.json()["email"] == unique_user["email"]

        # Generate API key
        r = requests.post(f"{base_url}/auth/key", headers=headers)
        assert r.status_code == 200
        api_key = r.json()["token"]
        assert len(api_key) > 0

        # Use API key to hit /auth/me
        r = requests.get(
            f"{base_url}/auth/me",
            headers={"Authorization": f"Bearer {api_key}"},
        )
        assert r.status_code == 200
        assert r.json()["email"] == unique_user["email"]


# ---------------------------------------------------------------------------
# 2. Duplicate signup
# ---------------------------------------------------------------------------


class TestDuplicateSignup:
    def test_duplicate_email_rejected(self, base_url, unique_user):
        r = requests.post(
            f"{base_url}/auth/signup",
            json={"email": unique_user["email"], "password": "AnotherPass1!"},
        )
        assert r.status_code in (409, 500), f"Expected conflict, got {r.status_code}"


# ---------------------------------------------------------------------------
# 3. Unauthorized access
# ---------------------------------------------------------------------------


class TestUnauthorized:
    @pytest.mark.parametrize(
        "method,path",
        [
            ("GET", "/auth/me"),
            ("POST", "/auth/key"),
            ("GET", "/manage/providers"),
            ("POST", "/manage/providers"),
            ("GET", "/manage/aliases"),
            ("POST", "/manage/aliases"),
            ("GET", "/manage/usage"),
        ],
    )
    def test_protected_endpoints_require_auth(self, base_url, method, path):
        r = requests.request(method, f"{base_url}{path}")
        assert r.status_code == 401, f"{method} {path} returned {r.status_code}"


# ---------------------------------------------------------------------------
# 4. Provider keys
# ---------------------------------------------------------------------------


class TestProviderKeys:
    def test_create_and_list(self, base_url, unique_user, provider_key):
        providers = provider_key
        pk = providers[0]
        assert pk["provider"] == "openai"
        assert pk["label"] == "E2E Test Key"
        assert "id" in pk
        assert "created_at" in pk
        # encrypted key should NOT be returned
        assert "encrypted_key" not in pk


# ---------------------------------------------------------------------------
# 5. Alias CRUD
# ---------------------------------------------------------------------------


class TestAliasCRUD:
    def test_create_and_list(self, base_url, unique_user, provider_key):
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]

        alias_name = f"test-alias-{uuid.uuid4().hex[:8]}"
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
            },
        )
        assert r.status_code == 200, f"Create alias failed: {r.text}"

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        assert r.status_code == 200
        aliases = r.json()
        match = [a for a in aliases if a["alias"] == alias_name]
        assert len(match) == 1
        a = match[0]
        assert a["target_model"] == "gpt-4"
        assert a["provider_key_id"] == pk_id
        assert "id" in a


# ---------------------------------------------------------------------------
# 6. Alias with null optionals
# ---------------------------------------------------------------------------


class TestAliasNullOptionals:
    def test_explicit_nulls(self, base_url, unique_user, provider_key):
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]
        alias_name = f"null-opt-{uuid.uuid4().hex[:8]}"

        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
                "fallback_alias_id": None,
                "light_model": None,
            },
        )
        assert r.status_code == 200

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        aliases = r.json()
        match = [a for a in aliases if a["alias"] == alias_name][0]
        assert match["fallback_alias_id"] is None
        assert match["light_model"] is None


# ---------------------------------------------------------------------------
# 7. Alias with zero/empty optionals (bug regression)
# ---------------------------------------------------------------------------


class TestAliasZeroEmptyOptionals:
    def test_zero_and_empty_normalized_to_null(self, base_url, unique_user, provider_key):
        """Backend should normalize fallback_alias_id=0 and light_model='' to null."""
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]
        alias_name = f"zero-opt-{uuid.uuid4().hex[:8]}"

        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
                "fallback_alias_id": 0,
                "light_model": "",
            },
        )
        assert r.status_code == 200

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        aliases = r.json()
        match = [a for a in aliases if a["alias"] == alias_name][0]
        assert match["fallback_alias_id"] is None, (
            f"Expected null, got {match['fallback_alias_id']}"
        )
        assert match["light_model"] is None, (
            f"Expected null, got {match['light_model']}"
        )


# ---------------------------------------------------------------------------
# 8. Alias upsert
# ---------------------------------------------------------------------------


class TestAliasUpsert:
    def test_upsert_updates_existing(self, base_url, unique_user, provider_key):
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]
        alias_name = f"upsert-{uuid.uuid4().hex[:8]}"

        # Create
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
            },
        )
        assert r.status_code == 200

        # Upsert with different target
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4-turbo",
                "provider_key_id": pk_id,
            },
        )
        assert r.status_code == 200

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        aliases = r.json()
        match = [a for a in aliases if a["alias"] == alias_name]
        assert len(match) == 1, "Upsert should not create a duplicate"
        assert match[0]["target_model"] == "gpt-4-turbo"


# ---------------------------------------------------------------------------
# 9. Alias PATCH
# ---------------------------------------------------------------------------


class TestAliasPatch:
    def test_patch_specific_fields(self, base_url, unique_user, provider_key):
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]
        alias_name = f"patch-{uuid.uuid4().hex[:8]}"

        # Create
        requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
            },
        )

        # Patch target_model
        r = requests.patch(
            f"{base_url}/manage/aliases/{alias_name}",
            headers=headers,
            json={"target_model": "gpt-4o"},
        )
        assert r.status_code == 200

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        aliases = r.json()
        match = [a for a in aliases if a["alias"] == alias_name][0]
        assert match["target_model"] == "gpt-4o"


# ---------------------------------------------------------------------------
# 10. Alias with advanced options
# ---------------------------------------------------------------------------


class TestAliasAdvanced:
    def test_light_model_and_fallback(self, base_url, unique_user, provider_key):
        headers = unique_user["headers"]
        pk_id = provider_key[0]["id"]

        # Create a fallback alias first
        fallback_name = f"fb-{uuid.uuid4().hex[:8]}"
        requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": fallback_name,
                "target_model": "gpt-3.5-turbo",
                "provider_key_id": pk_id,
            },
        )

        # Get its ID
        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        fb = [a for a in r.json() if a["alias"] == fallback_name][0]
        fb_id = fb["id"]

        # Create alias with all advanced options
        alias_name = f"adv-{uuid.uuid4().hex[:8]}"
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=headers,
            json={
                "alias": alias_name,
                "target_model": "gpt-4",
                "provider_key_id": pk_id,
                "fallback_alias_id": fb_id,
                "use_light_model": True,
                "light_model_threshold": 50,
                "light_model": "gpt-3.5-turbo",
            },
        )
        assert r.status_code == 200

        r = requests.get(f"{base_url}/manage/aliases", headers=headers)
        match = [a for a in r.json() if a["alias"] == alias_name][0]
        assert match["fallback_alias_id"] == fb_id
        assert match["use_light_model"] is True
        assert match["light_model_threshold"] == 50
        assert match["light_model"] == "gpt-3.5-turbo"


# ---------------------------------------------------------------------------
# 11. Alias validation
# ---------------------------------------------------------------------------


class TestAliasValidation:
    def test_missing_alias_name(self, base_url, unique_user, provider_key):
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=unique_user["headers"],
            json={
                "target_model": "gpt-4",
                "provider_key_id": provider_key[0]["id"],
            },
        )
        assert r.status_code == 400

    def test_missing_target_model(self, base_url, unique_user, provider_key):
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=unique_user["headers"],
            json={
                "alias": "val-test",
                "provider_key_id": provider_key[0]["id"],
            },
        )
        assert r.status_code == 400

    def test_missing_provider_key(self, base_url, unique_user):
        r = requests.post(
            f"{base_url}/manage/aliases",
            headers=unique_user["headers"],
            json={
                "alias": "val-test",
                "target_model": "gpt-4",
            },
        )
        assert r.status_code == 400


# ---------------------------------------------------------------------------
# 12. Usage stats
# ---------------------------------------------------------------------------


class TestUsageStats:
    def test_usage_returns_list(self, base_url, unique_user):
        r = requests.get(
            f"{base_url}/manage/usage",
            headers=unique_user["headers"],
        )
        assert r.status_code == 200
        # New user should have null (no rows) or empty list
        data = r.json()
        assert data is None or isinstance(data, list)
