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
Library           ${WEB_SERVICE_LIBRARY}          ${CWD_FOR_EXEC}    
...               tls_key_path=${REPOSITORY_ROOT}${/}test${/}credentials${/}pg_server_key.pem
...               tls_cert_path=${REPOSITORY_ROOT}${/}test${/}credentials${/}pg_server_cert.pem
...               log_root=${REPOSITORY_ROOT}${/}test${/}robot${/}cli${/}mocked${/}log

*** Keywords ***
Start All Mock Servers
    ${port_dict} =    Create Dictionary    
    ...    oauth_client_credentials_token=${MOCKSERVER_PORT_OAUTH_CLIENT_CREDENTIALS_TOKEN}
    ...    github=${MOCKSERVER_PORT_GITHUB}
    ...    google=${MOCKSERVER_PORT_GOOGLE}
    ...    okta=${MOCKSERVER_PORT_OKTA}
    ...    aws=${MOCKSERVER_PORT_AWS}
    ...    stackql_auth_testing=${MOCKSERVER_PORT_STACKQL_AUTH_TESTING}
    ...    googleadmin=${MOCKSERVER_PORT_GOOGLEADMIN}
    ...    k8s=${MOCKSERVER_PORT_K8S}
    ...    registry=${MOCKSERVER_PORT_REGISTRY}
    ...    azure=${MOCKSERVER_PORT_AZURE}
    ...    sumologic=${MOCKSERVER_PORT_SUMOLOGIC}
    ...    digitalocean=${MOCKSERVER_PORT_DIGITALOCEAN}
    Start All Webservers    port_dict=${port_dict}

Prepare StackQL Environment
    [Documentation]    Prepare StackQL Environment
    Start All Mock Servers
    Sleep    50s

Stackql Per Test Teardown
    [Documentation]    Stackql Per Test Teardown
    Log                Stackql Per Test Teardown Placeholder run
