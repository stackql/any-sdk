
FROM python:3.14.2-trixie AS utility

RUN --mount=type=bind,source=cicd/testlib-requirements.txt,target=/tmp/requirements.txt \
    pip install --requirement /tmp/requirements.txt \
    && mkdir -p /opt/testlib/test/python \
    && mkdir -p /opt/testlib/cicd/vol/srv/credentials

RUN --mount=type=bind,source=cicd/testlib-requirements.txt,target=/tmp/requirements.txt \
    pip install --requirement /tmp/requirements.txt

RUN apt-get update \
    && apt-get install -y ca-certificates openssl netcat-traditional jq dnsutils \
    && update-ca-certificates

CMD ["python", "-c", "print('Hello from testlib')"]

