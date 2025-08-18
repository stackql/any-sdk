*** Settings ***
Resource          ${CURDIR}/stackql_aot.resource
Suite Setup       Prepare AOT StackQL Environment
Suite Teardown    Terminate All Processes    kill=True

