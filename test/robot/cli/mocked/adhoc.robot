*** Settings ***
Resource          ${CURDIR}/anysdk_mocked.resource
Test Teardown     AnySDK Per Test Teardown

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
