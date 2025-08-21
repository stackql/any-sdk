*** Variables ***
${REPOSITORY_ROOT}                     ${CURDIR}${/}..${/}..${/}..${/}..
${LOCAL_LIB_HOME}                      ${REPOSITORY_ROOT}${/}test${/}python
${EXECUTION_PLATFORM}                  native   # to be overridden from command line, eg "docker"
${SQL_BACKEND}                         sqlite_embedded   # to be overridden from command line, eg "postgres_tcp"
${IS_WSL}                              false   # to be overridden from command line, with string "true"
${SHOULD_RUN_DOCKER_EXTERNAL_TESTS}    false   # to be overridden from command line, with string "true"
${CONCURRENCY_LIMIT}                   1       # to be overridden from command line, with integer value, -1 for no limit
${USE_STACKQL_PREINSTALLED}            false   # to be overridden from command line, with string "true"
${SUNDRY_CONFIG}                       {}  # to be overridden from command line, with string value
${CORE_PREFIX}                         stackql-core
${CWD_FOR_EXEC}                        ${REPOSITORY_ROOT}  # works for self repository, can be overwritten when shared
${WEB_SERVICE_LIBRARY}                 stackql_test_tooling.web_service_keywords
${STACKQL_INTERFACE_LIBRARY}           stackql_test_tooling.StackQLInterfaces
${CLOUD_INTEGRATION_LIBRARY}           stackql_test_tooling.CloudIntegration
${CLI_EXE}                             ${REPOSITORY_ROOT}${/}build${/}anysdk
${MOCKSERVER_PORT_OAUTH_CLIENT_CREDENTIALS_TOKEN}    2091
${MOCKSERVER_PORT_GITHUB}                            1093                    
${MOCKSERVER_PORT_GOOGLE}                            1080
${MOCKSERVER_PORT_OKTA}                              1090
${MOCKSERVER_PORT_AWS}                               1091
${MOCKSERVER_PORT_STACKQL_AUTH_TESTING}              1170
${MOCKSERVER_PORT_GOOGLEADMIN}                       1098
${MOCKSERVER_PORT_K8S}                               1092
${MOCKSERVER_PORT_REGISTRY}                          1094
${MOCKSERVER_PORT_AZURE}                             1095
${MOCKSERVER_PORT_SUMOLOGIC}                         1096
${MOCKSERVER_PORT_DIGITALOCEAN}                      1097


*** Settings ***
Library           Process
Library           OperatingSystem
Library           Process
Library           OperatingSystem
Library           String
Library           ${STACKQL_INTERFACE_LIBRARY}    ${EXECUTION_PLATFORM}    ${SQL_BACKEND}    ${CONCURRENCY_LIMIT}
Library           ${CLOUD_INTEGRATION_LIBRARY}

*** Keywords ***
Prepare AnySDK Environment
    [Documentation]    Prepare AnySDK Environment
    Log                AnySDK Per Test Teardown Placeholder run
    Sleep    2s

AnySDK Per Test Teardown
    [Documentation]    AnySDK Per Test Teardown
    Log                AnySDK Per Test Teardown Placeholder run
