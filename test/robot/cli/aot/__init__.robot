*** Settings ***
Resource          ${CURDIR}/anysdk_aot.resource
Suite Setup       Prepare AnySDK Environment
Suite Teardown    Terminate All Processes    kill=True

