*** Settings ***
Resource          ${CURDIR}/anysdk_interrogate.resource
Test Teardown     AnySDK Per Test Teardown

*** Test Cases *** 
Simple Interrogate Services Fail
    [Documentation]    Test Interrogate Services CLI Fails as Expected
    [Tags]    cli    interrogate
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    interrogate
    ...    services
    ...    \./test/registry
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-Interrogate-Services-Fail.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-Interrogate-Services-Fail_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    Should Be Equal                  ${result.stderr}    
    ...                              inoperable input; expected 'interrogate services <path to registry root> <path to provider doc>'
    Should Be Equal As Strings    ${result.rc}    1

Simple Interrogate Services Success
    [Documentation]    Test Interrogate Services CLI Working
    [Tags]    cli    interrogate
    ${result} =    Run Process
    ...    ${CLI_EXE}
    ...    interrogate
    ...    services
    ...    \./test/registry
    ...    \./test/registry/src/googleapis\.com/v0\.1\.2/provider\.yaml
    ...    cwd=${CWD_FOR_EXEC}
    ...    stdout=${CURDIR}${/}/tmp${/}Simple-Interrogate-Services-Success.txt
    ...    stderr=${CURDIR}${/}/tmp${/}Simple-Interrogate-Services-Success_stderr.txt
    Log    Stderr = ${result.stderr}
    Log    Stdout = ${result.stdout}
    Log    RC = ${result.rc}
    ${expected_stdout}=    Get File    ${CURDIR}${/}expectations${/}Simple-Interrogate-Services-Success.txt
    Should Be Equal                  ${result.stdout}    
    ...                              ${expected_stdout}
    Should Be Equal As Strings    ${result.rc}    0

