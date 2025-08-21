*** Settings ***
Resource          ${CURDIR}/anysdk_interrogate.resource
Suite Setup       Prepare AnySDK Environment
Suite Teardown    Terminate All Processes    kill=True

