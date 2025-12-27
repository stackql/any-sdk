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

Update Google Cloud Storage Bucket ABAC with CLI Demonstrates Request Body Rewrite
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
    ...    \-\-parameters             "{ "region": "ap\-southeast\-2", "Bucket": "stackql\-trial\-bucket\-02", "Status": "Enabled" }
    ...    \-\-auth                   {"google": {"credentialsenvvar": "GCP_SERVICE_ACCOUNT_KEY"}}
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Update-Google-Cloud-Storage-Bucket-ABAC-with-CLI-Demonstrates-Request-Body-Rewrite.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Update-Google-Cloud-Storage-Bucket-ABAC-with-CLI-Demonstrates-Request-Body-Rewrite_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Be Equal As Strings    ${result.rc}    0
    Should Be Equal               ${result.stdout}        ${EMPTY}
    Should Be Equal               ${result.stderr}        ${EMPTY}
