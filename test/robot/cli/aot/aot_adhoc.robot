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
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-Google-Provider-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-Google-Provider-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stderr}    
    ...                                error count 564
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
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-AWS-Provider-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Analysis-AWS-Provider-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stdout}    
    ...                                successfully performed AOT analysis
    Should Be Equal As Strings    ${result.rc}    0

Simple AOT Service Level Analysis AWS EC2 with CLI
    [Documentation]    Test CLI Working
    [Tags]    cli    aot
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    aot
    ...    \./test/registry
    ...    \./test/registry/src/aws/v0\.1\.0/provider\.yaml
    ...    ec2
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-AOT-Service-Level-Analysis-AWS-EC2-with-CLI.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-AOT-Service-Level-Analysis-AWS-EC2-with-CLI_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Contain                     ${result.stderr}    
    ...                                successfully dereferenced method = 'describeVolumes' for resource = 'volumes' with service name = 'ec2'
    Should Be Equal As Strings    ${result.rc}    0
