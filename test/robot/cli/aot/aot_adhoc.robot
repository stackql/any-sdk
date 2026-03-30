*** Settings ***
Resource          ${CURDIR}/anysdk_aot.resource
Test Teardown     AnySDK Per Test Teardown

*** Test Cases *** 
Simple AOT Analysis Google Provider with CLI
    [Documentation]    Test CLI Working
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \./test/registry
    ...    \./test/registry/src/googleapis\.com/v0\.1\.2/provider\.yaml
    ...    \-\-schema-dir 
    ...    cicd/schema-definitions
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-Google-Provider-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-Google-Provider-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stderr}    
    ...                                missing-semantics
    Should Be Equal As Strings    ${result.rc}    1

Simple AOT Analysis AWS Provider with CLI
    [Documentation]    Test CLI Working
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \./test/registry
    ...    \./test/registry/src/aws/v0\.1\.0/provider\.yaml
    ...    \-v
    ...    \-\-schema-dir 
    ...    cicd/schema-definitions
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-AWS-Provider-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-AWS-Provider-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    # Should Contain                     ${result.stdout}    
    # ...                                successfully performed AOT analysis
    Should Contain                     ${result.stderr}    
    ...                                missing-semantics
    Should Be Equal As Strings    ${result.rc}    1

Simple AOT Service Level Analysis AWS EC2 with CLI
    [Documentation]    Test CLI Working
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \-v
    ...    \./test/registry
    ...    \./test/registry/src/aws/v0\.1\.0/provider\.yaml
    ...    ec2
    ...    \-\-schema-dir
    ...    cicd/schema-definitions
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Service-Level-Analysis-AWS-EC2-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Service-Level-Analysis-AWS-EC2-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stderr}
    ...                                successfully dereferenced method = 'describeVolumes' for resource = 'volumes' with service name = 'ec2'
    Should Be Equal As Strings    ${result.rc}    1

AOT Resource Level Analysis AWS EC2 volumes_post_naively_presented with CLI
    [Documentation]    Test resource level AOT analysis produces structured findings with prior and fixed templates
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \-v
    ...    \./test/registry
    ...    \./test/registry/src/aws/v0\.1\.0/provider\.yaml
    ...    ec2
    ...    \-\-provider
    ...    aws
    ...    \-\-resource
    ...    volumes_post_naively_presented
    ...    \-\-schema-dir
    ...    cicd/schema-definitions
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}AOT-Resource-Level-Analysis-AWS-EC2-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}AOT-Resource-Level-Analysis-AWS-EC2-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    # stderr: JSONL findings include classification bins, template fix pair, and verbose info messages
    Should Contain                     ${result.stderr}
    ...                                "bin":"empty-response-unsafe"
    Should Contain                     ${result.stderr}
    ...                                "bin":"missing-semantics"
    Should Contain                     ${result.stderr}
    ...                                "prior_template":"{{ toJson . }}"
    Should Contain                     ${result.stderr}
    ...                                "fixed_template":"{{- if . -}}{{ toJson . }}{{- else -}}null{{- end -}}"
    Should Contain                     ${result.stderr}
    ...                                successfully built HTTP request context for method 'POST_DescribeVolumes' on resource 'volumes_post_naively_presented'
    Should Contain                     ${result.stderr}
    ...                                successfully dereferenced method = 'describeVolumes' for resource = 'volumes_post_naively_presented' with service name = 'ec2'
    Should Contain                     ${result.stderr}
    ...                                response transform template parses successfully
    # stdout: JSON summary with correct totals, bins, and service breakdown
    Should Contain                     ${result.stdout}
    ...                                "total_warnings": 2
    Should Contain                     ${result.stdout}
    ...                                "total_errors": 0
    Should Contain                     ${result.stdout}
    ...                                "empty-response-unsafe"
    Should Contain                     ${result.stdout}
    ...                                "missing-semantics"
    Should Contain                     ${result.stdout}
    ...                                "warning_count": 2
    Should Contain                     ${result.stdout}
    ...                                sample_response
    Should Be Equal As Strings    ${result.rc}    0

AOT Method Level Analysis AWS EC2 volumes_post_naively_presented describeVolumes with CLI
    [Documentation]    Test method level AOT analysis produces structured findings scoped to a single method
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \-v
    ...    \./test/registry
    ...    \./test/registry/src/aws/v0\.1\.0/provider\.yaml
    ...    ec2
    ...    \-\-provider
    ...    aws
    ...    \-\-resource
    ...    volumes_post_naively_presented
    ...    \-\-method
    ...    describeVolumes
    ...    \-\-schema-dir
    ...    cicd/schema-definitions
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}AOT-Method-Level-Analysis-AWS-EC2-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}AOT-Method-Level-Analysis-AWS-EC2-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    # stderr: JSONL findings scoped to the specific method
    Should Contain                     ${result.stderr}
    ...                                "bin":"empty-response-unsafe"
    Should Contain                     ${result.stderr}
    ...                                "method":"describeVolumes"
    Should Contain                     ${result.stderr}
    ...                                "prior_template":"{{ toJson . }}"
    Should Contain                     ${result.stderr}
    ...                                "fixed_template":"{{- if . -}}{{ toJson . }}{{- else -}}null{{- end -}}"
    Should Contain                     ${result.stderr}
    ...                                successfully built HTTP request context for method 'POST_DescribeVolumes' on resource 'volumes_post_naively_presented'
    Should Contain                     ${result.stderr}
    ...                                successfully inferred response schema for method = 'describeVolumes'
    # stdout: JSON summary with method-level detail
    Should Contain                     ${result.stdout}
    ...                                "total_warnings": 2
    Should Contain                     ${result.stdout}
    ...                                "total_errors": 0
    Should Contain                     ${result.stdout}
    ...                                "empty-response-unsafe"
    Should Contain                     ${result.stdout}
    ...                                "missing-semantics"
    Should Contain                     ${result.stdout}
    ...                                "method": "describeVolumes"
    Should Contain                     ${result.stdout}
    ...                                sample_response
    Should Be Equal As Strings    ${result.rc}    0
