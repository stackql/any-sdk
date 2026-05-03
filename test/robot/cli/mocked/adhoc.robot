*** Settings ***
Resource          ${CURDIR}/anysdk_mocked.resource
Test Teardown     AnySDK Per Test Teardown

*** Variables ***
${RETRY_MOCK_HOST}      127.0.0.1
${RETRY_MOCK_PORT}      1199
${RETRY_MOCK_BASE}      http://${RETRY_MOCK_HOST}:${RETRY_MOCK_PORT}
${RETRY_PROV_PATH}      test/registry-mocked/src/retrytestprovider/v0.1.0/provider.yaml
${RETRY_SVC_PATH}       test/registry-mocked/src/retrytestprovider/v0.1.0/services/flaky.yaml
${RETRY_AUTH_JSON}      {"retrytestprovider": {"type": "null_auth"}}

*** Test Cases *** 
Select Google Cloud Storage Buckets with CLI
    [Documentation]    Test CLI Working
    [Tags]    cli
    ${google_credentials} =    Get File    ${REPOSITORY_ROOT}${/}test${/}assets${/}credentials${/}dummy${/}google${/}functional-test-dummy-sa-key.json
    Set Environment Variable    GCP_SERVICE_ACCOUNT_KEY    ${google_credentials}
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-svc-file-path          test/registry\-mocked/src/googleapis.com/v0\.1\.2/services/storage\-v1\.yaml
    ...    \-\-tls.allowInsecure
    ...    \-\-prov-file-path         test/registry\-mocked/src/googleapis\.com/v0\.1\.2/provider\.yaml
    ...    \-\-resource               buckets
    ...    \-\-method                 list
    ...    \-\-parameters             {"project": "stackql\-demo"} 
    ...    \-\-auth                   {"google": {"credentialsenvvar": "GCP_SERVICE_ACCOUNT_KEY"}}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Select-Google-Cloud-Storage-Buckets-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Select-Google-Cloud-Storage-Buckets-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stdout}    
    ...                                stackql\-demo
    Should Be Equal As Strings    ${result.rc}    0
    Should Be Equal               ${result.stderr}        ${EMPTY}

Update AWS Bucket ABAC with CLI Demonstrates Request Body Rewrite
    [Documentation]    Test CLI Working
    [Tags]    cli
    ${google_credentials} =    Get File    ${REPOSITORY_ROOT}${/}test${/}assets${/}credentials${/}dummy${/}google${/}functional-test-dummy-sa-key.json
    Set Environment Variable    GCP_SERVICE_ACCOUNT_KEY    ${google_credentials}
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-svc-file-path          test/registry\-mocked/src/aws/v0\.1\.0/services/s3\.yaml
    ...    \-\-tls.allowInsecure
    ...    \-\-prov-file-path         test/registry\-mocked/src/aws/v0\.1\.0/provider\.yaml
    ...    \-\-resource               bucket_abac
    ...    \-\-method                 put_bucket_abac
    ...    \-\-parameters             { "region": "ap-southeast-2", "Bucket": "stackql-trial-bucket-02", "Status": "Enabled" }
    ...    \-\-auth                   {"google": {"credentialsenvvar": "GCP_SERVICE_ACCOUNT_KEY"}}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Update-AWS-Bucket-ABAC-with-CLI-Demonstrates-Request-Body-Rewrite.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Update-AWS-Bucket-ABAC-with-CLI-Demonstrates-Request-Body-Rewrite_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Be Equal As Strings    ${result.rc}    0
    Should Be Equal               ${result.stdout}        ${EMPTY}
    Should Be Equal               ${result.stderr}        ${EMPTY}

AWS EC2 Describe Volumes Demonstrates No Request Body Parameters Still Expands Template
    [Documentation]    Test CLI Working
    [Tags]    cli
    ${google_credentials} =    Get File    ${REPOSITORY_ROOT}${/}test${/}assets${/}credentials${/}dummy${/}google${/}functional-test-dummy-sa-key.json
    Set Environment Variable    GCP_SERVICE_ACCOUNT_KEY    ${google_credentials}
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-svc-file-path          test/registry\-mocked/src/aws/v0\.1\.0/services/ec2\.yaml
    ...    \-\-tls.allowInsecure
    ...    \-\-prov-file-path         test/registry\-mocked/src/aws/v0\.1\.0/provider\.yaml
    ...    \-\-resource               volumes_post_naively_presented
    ...    \-\-method                 describeVolumes
    ...    \-\-parameters             { "region": "ap-southeast-2" }
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}AWS-EC2-Describe-Volumes-Demonstrates-No-Request-Body-Parameters-Still-Expands-Template.txt
    ...    stderr=${CURDIR}${/}/tmp${/}AWS-EC2-Describe-Volumes-Demonstrates-No-Request-Body-Parameters-Still-Expands-Template_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Be Equal As Strings    ${result.rc}    0
    Should Contain                     ${result.stdout}
    ...                                vol\-00100000000000000
    Should Be Equal               ${result.stderr}        ${EMPTY}

Default Retry Policy Recovers After Transient 503s
    [Documentation]    Default policy (3 attempts) — server fails twice then succeeds.
    [Tags]    cli    retry
    Ensure Retry Mock Running
    Reset Retry Mock Counters
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-prov-file-path         ${RETRY_PROV_PATH}
    ...    \-\-svc-file-path          ${RETRY_SVC_PATH}
    ...    \-\-resource               recoverable_default
    ...    \-\-method                 get
    ...    \-\-parameters             {"fail_until": 2}
    ...    \-\-auth                   ${RETRY_AUTH_JSON}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Default-Retry-Policy-Recovers.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Default-Retry-Policy-Recovers_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Should Be Equal As Strings    ${result.rc}    0
    Should Contain                ${result.stdout}    "ok":true
    Should Contain                ${result.stdout}    "attempt":3
    Assert Mock Attempts          default-recover    3

Configured Retry Policy Recovers On Fifth Attempt
    [Documentation]    Resource-level config (max_attempts=5) — server fails four times then succeeds.
    [Tags]    cli    retry
    Ensure Retry Mock Running
    Reset Retry Mock Counters
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-prov-file-path         ${RETRY_PROV_PATH}
    ...    \-\-svc-file-path          ${RETRY_SVC_PATH}
    ...    \-\-resource               recoverable_configured
    ...    \-\-method                 get
    ...    \-\-parameters             {"fail_until": 4}
    ...    \-\-auth                   ${RETRY_AUTH_JSON}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Configured-Retry-Policy-Recovers.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Configured-Retry-Policy-Recovers_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Should Be Equal As Strings    ${result.rc}    0
    Should Contain                ${result.stdout}    "ok":true
    Should Contain                ${result.stdout}    "attempt":5
    Assert Mock Attempts          configured-recover    5

Zero Retry Policy Issues Exactly One Attempt
    [Documentation]    Resource-level config max_attempts=1 disables retry entirely.
    [Tags]    cli    retry
    Ensure Retry Mock Running
    Reset Retry Mock Counters
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-prov-file-path         ${RETRY_PROV_PATH}
    ...    \-\-svc-file-path          ${RETRY_SVC_PATH}
    ...    \-\-resource               no_retry
    ...    \-\-method                 get
    ...    \-\-auth                   ${RETRY_AUTH_JSON}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Zero-Retry-Single-Attempt.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Zero-Retry-Single-Attempt_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Should Contain                ${result.stdout}    "ok":false
    Should Contain                ${result.stdout}    "attempt":1
    Should Contain                ${result.stderr}    503
    Assert Mock Attempts          always_503    1

Tight Retry Budget Surfaces Final 503
    [Documentation]    Resource-level config max_attempts=2 with four required failures — should exhaust.
    [Tags]    cli    retry
    Ensure Retry Mock Running
    Reset Retry Mock Counters
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    query
    ...    \-\-prov-file-path         ${RETRY_PROV_PATH}
    ...    \-\-svc-file-path          ${RETRY_SVC_PATH}
    ...    \-\-resource               tight_budget
    ...    \-\-method                 get
    ...    \-\-parameters             {"fail_until": 4}
    ...    \-\-auth                   ${RETRY_AUTH_JSON}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Tight-Retry-Budget.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Tight-Retry-Budget_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Should Contain                ${result.stdout}    "ok":false
    Should Contain                ${result.stdout}    "attempt":2
    Should Contain                ${result.stderr}    503
    Assert Mock Attempts          tight-budget    2

*** Keywords ***
Ensure Retry Mock Running
    [Documentation]    Confirm the local flask retry mock is reachable. CI starts it
    ...                ahead of time; for local dev we'll spin it up on demand.
    ${ping} =    Run Process    curl    -sf    -X    POST    ${RETRY_MOCK_BASE}/reset
    Log    Ping rc=${ping.rc} stdout=${ping.stdout} stderr=${ping.stderr}
    IF    '${ping.rc}' == '0'    RETURN
    Create Directory                  ${CURDIR}${/}tmp
    Start Process    flask    --app\=test/python/any_sdk_test_utils/mocks/retry_app:app    run    --host    ${RETRY_MOCK_HOST}    --port    ${RETRY_MOCK_PORT}
    ...    cwd=${CWD_FOR_EXEC}
    ...    alias=retry_mock_server
    ...    stdout=${CURDIR}${/}tmp${/}retry_mock_stdout.log
    ...    stderr=${CURDIR}${/}tmp${/}retry_mock_stderr.log
    ${started} =    Run Keyword And Return Status    Wait Until Keyword Succeeds    60x    500ms    Reset Retry Mock Counters
    IF    not ${started}    Log Retry Mock Diagnostics
    Should Be True    ${started}    Retry mock did not become reachable on ${RETRY_MOCK_BASE}

Log Retry Mock Diagnostics
    ${stdout_exists} =    Run Keyword And Return Status    File Should Exist    ${CURDIR}${/}tmp${/}retry_mock_stdout.log
    ${stderr_exists} =    Run Keyword And Return Status    File Should Exist    ${CURDIR}${/}tmp${/}retry_mock_stderr.log
    IF    ${stdout_exists}    Log File    ${CURDIR}${/}tmp${/}retry_mock_stdout.log
    IF    ${stderr_exists}    Log File    ${CURDIR}${/}tmp${/}retry_mock_stderr.log

Reset Retry Mock Counters
    ${reset} =    Run Process    curl    -sf    -X    POST    ${RETRY_MOCK_BASE}/reset
    Should Be Equal As Strings    ${reset.rc}    0    Reset call to ${RETRY_MOCK_BASE}/reset returned rc=${reset.rc} stdout='${reset.stdout}' stderr='${reset.stderr}'

Assert Mock Attempts
    [Arguments]    ${key}    ${expected}
    ${count} =    Run Process    curl    -sf    ${RETRY_MOCK_BASE}/count/${key}
    Should Be Equal As Strings    ${count.rc}    0
    Should Contain                ${count.stdout}    "attempts":${expected}
