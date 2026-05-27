# Identity Federation in stackql — setup brief

Covers the three federation auth types added in any-sdk: `aws_web_identity`,
`gcp_workload_identity`, `azure_federated`. Each takes a foreign OIDC token
(from GitHub Actions, k8s projected SA, etc.) and exchanges it at the target
cloud's STS/token endpoint for short-lived cloud credentials. No long-lived
secrets.

Shared `AuthCtx` knobs:

- `oidc_subject_token_file` — path; **re-read on every refresh** (rotation works).
- `oidc_subject_token_file_env_var` — env var holding the path.
- `oidc_subject_token` — inline; tests only.

---

## 1) Live setup using GitHub Actions OIDC (CI)

Every job that federates needs:

```yaml
permissions:
  id-token: write   # required to mint the OIDC token
  contents: read
```

And a step that fetches the token and writes it to a file stackql can read:

```yaml
- name: Mint GHA OIDC token
  shell: bash
  env:
    AUDIENCE: ${{ env.GHA_OIDC_AUDIENCE }}     # per-cloud, see below
  run: |
    curl -sLS "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=${AUDIENCE}" \
      -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
      | jq -r '.value' > /tmp/gha-oidc-token
    echo "OIDC_SUBJECT_TOKEN_FILE=/tmp/gha-oidc-token" >> "$GITHUB_ENV"
```

Then point a stackql auth block at `/tmp/gha-oidc-token`.

### AWS

**One-time cloud setup**

1. IAM → Identity providers → Add OIDC provider:
   - URL: `https://token.actions.githubusercontent.com`
   - Audience: `sts.amazonaws.com`
2. Create an IAM role with a trust policy bound to that provider:
   ```json
   {
     "Effect": "Allow",
     "Principal": { "Federated": "arn:aws:iam::ACCT:oidc-provider/token.actions.githubusercontent.com" },
     "Action": "sts:AssumeRoleWithWebIdentity",
     "Condition": {
       "StringEquals": { "token.actions.githubusercontent.com:aud": "sts.amazonaws.com" },
       "StringLike":   { "token.actions.githubusercontent.com:sub": "repo:OWNER/REPO:*" }
     }
   }
   ```
3. Attach the AWS permissions the role needs (S3, EC2, etc.).

**GHA audience:** `sts.amazonaws.com`

**stackql auth JSON**
```json
{"aws":{
  "type":"aws_web_identity",
  "aws_role_arn":"arn:aws:iam::ACCT:role/MyGhaRole",
  "aws_sts_region":"us-east-1",
  "oidc_subject_token_file_env_var":"OIDC_SUBJECT_TOKEN_FILE"
}}
```

### GCP

**One-time cloud setup**

1. Create Workload Identity Pool + OIDC Provider:
   - Issuer: `https://token.actions.githubusercontent.com`
   - Allowed audiences: the pool-provider resource URL (default) **or** a
     custom audience you'll pass to GHA.
   - Attribute mapping (minimum): `google.subject = assertion.sub`,
     `attribute.repository = assertion.repository`.
2. Bind a service account: grant the principal
   `principalSet://iam.googleapis.com/projects/NUM/locations/global/workloadIdentityPools/POOL/attribute.repository/OWNER/REPO`
   the role `roles/iam.workloadIdentityUser` on the SA.
3. Grant the SA the GCP roles it needs.

**GHA audience:**
`//iam.googleapis.com/projects/NUM/locations/global/workloadIdentityPools/POOL/providers/PROV`

**stackql auth JSON**
```json
{"google":{
  "type":"gcp_workload_identity",
  "gcp_workload_identity_audience":"//iam.googleapis.com/projects/NUM/locations/global/workloadIdentityPools/POOL/providers/PROV",
  "gcp_service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SA@PROJ.iam.gserviceaccount.com:generateAccessToken",
  "scopes":["https://www.googleapis.com/auth/cloud-platform"],
  "oidc_subject_token_file_env_var":"OIDC_SUBJECT_TOKEN_FILE"
}}
```

### Azure

**One-time cloud setup**

1. Entra ID → App registrations → create app (or use existing).
2. App → Certificates & secrets → **Federated credentials** → Add:
   - Issuer: `https://token.actions.githubusercontent.com`
   - Subject: `repo:OWNER/REPO:ref:refs/heads/main` (or environment/tag).
   - Audience: `api://AzureADTokenExchange`
3. Give the app the Azure RBAC roles it needs (e.g. Reader on a subscription).

**GHA audience:** `api://AzureADTokenExchange`

**stackql auth JSON**
```json
{"azure":{
  "type":"azure_federated",
  "azure_tenant_id":"<TENANT-GUID>",
  "client_id":"<APP-ID>",
  "scopes":["https://management.azure.com/.default"],
  "oidc_subject_token_file_env_var":"OIDC_SUBJECT_TOKEN_FILE"
}}
```

---

## 2) Flask mocks + robot tests

**Yes, all three are mockable** — every federation type now has an
endpoint-override field, so a Flask mock can stand in for the real STS/token
endpoint:

| Type | Override field |
|---|---|
| `aws_web_identity` | `aws_sts_endpoint` |
| `gcp_workload_identity` | `gcp_workload_identity_token_url` (+ `gcp_service_account_impersonation_url` for the impersonation hop) |
| `azure_federated` | `azure_federated_endpoint` |

Subject token: write any string to a temp file in the robot test and point
`oidc_subject_token_file` at it. The mock doesn't verify the JWT — it just
echoes back a canned credentials response. (For GCP, the externalaccount
client sends the token as `subject_token` form param; the mock can assert on
shape but doesn't need to cryptographically verify.)

### Suggested layout in stackql (mirrors existing `test/python/any_sdk_test_utils/mocks/` pattern in any-sdk)

```
test/python/any_sdk_test_utils/mocks/
  aws_sts_app.py          # POST / → AssumeRoleWithWebIdentityResponse XML
  gcp_sts_app.py          # POST /v1/token → token-exchange JSON
                          # POST /v1/projects/.../generateAccessToken → impersonation JSON
  entra_app.py            # POST /<tenant>/oauth2/v2.0/token → {access_token,token_type,expires_in}
test/robot/cli/mocked/
  identity_federation.robot
test/registry-mocked/src/aws-fed/      # mock provider yaml using aws_web_identity
test/registry-mocked/src/gcp-fed/      # similar
test/registry-mocked/src/azure-fed/    # similar
```

### Skeleton: AWS STS mock (Flask)

```python
from flask import Flask, request
app = Flask(__name__)

ASSUME_ROLE_WITH_WEB_IDENTITY = """<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleWithWebIdentityResult>
    <Credentials>
      <AccessKeyId>ASIA_MOCK</AccessKeyId>
      <SecretAccessKey>mock_secret</SecretAccessKey>
      <SessionToken>mock_session_token</SessionToken>
      <Expiration>2999-01-01T00:00:00Z</Expiration>
    </Credentials>
  </AssumeRoleWithWebIdentityResult>
</AssumeRoleWithWebIdentityResponse>"""

@app.post("/")
def sts():
    assert request.form["Action"] == "AssumeRoleWithWebIdentity"
    assert request.form["WebIdentityToken"]
    return ASSUME_ROLE_WITH_WEB_IDENTITY, 200, {"Content-Type": "text/xml"}
```

(The Entra mock returns
`{"access_token":"...","token_type":"Bearer","expires_in":3600}` JSON; the GCP
STS mock returns the standard token-exchange JSON and, if impersonation is
exercised, a second route returning
`{"accessToken":"...","expireTime":"..."}`.)

### Robot test sketch

```robot
*** Settings ***
Library    Process
Library    OperatingSystem

*** Test Cases ***
AWS Web Identity Federation Against Mocked STS
    Create File    ${TEMP_DIR}/oidc-token    fake-subject-jwt
    ${result}=    Run Process    ${CLI_EXE}    query
    ...    --svc-file-path     test/registry-mocked/src/aws/v0.1.0/services/s3.yaml
    ...    --prov-file-path    test/registry-mocked/src/aws/v0.1.0/provider.yaml
    ...    --resource          buckets
    ...    --method            list_buckets
    ...    --parameters        { "region": "us-east-1" }
    ...    --auth              {"aws":{"type":"aws_web_identity","aws_role_arn":"arn:aws:iam::123:role/r","aws_sts_endpoint":"http://localhost:1092","oidc_subject_token_file":"${TEMP_DIR}/oidc-token"}}
    ...    --tls.allowInsecure
    Should Be Equal As Integers    ${result.rc}    0
```

### Workflow wiring (mirror the existing mocked-tests job)

Each federation mock is just another Flask app started before the robot run,
same shape as the existing AWS/cloudasset/retry mocks in any-sdk:

```yaml
- name: Start federation mocks
  run: |
    python -m any_sdk_test_utils.mocks.aws_sts_app    &  # listens on 1092
    python -m any_sdk_test_utils.mocks.gcp_sts_app    &  # 1093
    python -m any_sdk_test_utils.mocks.entra_app      &  # 1094
- name: Run identity-federation robot tests
  env:
    OIDC_SUBJECT_TOKEN_FILE: ${{ runner.temp }}/oidc-token
  run: |
    echo "mock-subject-jwt" > "${OIDC_SUBJECT_TOKEN_FILE}"
    robot -d test/robot/reports/mocked-fed test/robot/cli/mocked/identity_federation.robot
```

---

## Quick references

- **Live federation:** needs `permissions: id-token: write` + per-cloud trust
  setup + GHA audience matching the cloud's expectation.
- **Mocked federation:** point the per-cloud endpoint-override field at a
  Flask app; the subject-token file can hold any string.
- The subject-token file is **re-read on every refresh**, so platform-rotated
  tokens (GHA, IRSA, projected k8s SA tokens) work out of the box.