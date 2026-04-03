# record-parser

Tiny example package.

## Build with uv

```bash
uv --directory cicd/python/record_parser build
```

## Install editable

```bash
uv venv && uv pip install -e ./cicd/python/record_parser
```

## Use

```python
from record_parser import parse_record, generate_flask_app, generate_flask_apps_from_file, generate_mocks_from_analysis_run


generate_mocks_from_analysis_run("test/assets/analysis-jsonl", "cicd/out/aot")


for line in generate_flask_apps_from_file("test/assets/analysis-jsonl/single-entry-observations.jsonl"): print(json.dumps(line))

```




## De facto protocol



```json
{
  "level": "warning",
  "bin": "empty-response-unsafe",
  "provider": "aws",
  "service": "ec2",
  "resource": "volumes_post_naively_presented",
  "method": "describeVolumes",
  "message": "response transform template accesses input directly without nil/empty guards — may fail on empty response bodies",
  "prior_template": "{{ toJson . }}",
  "fixed_template": "{{- if . -}}{{ toJson . }}{{- else -}}null{{- end -}}",
  "empirical_tests": {
    "results": [
      {
        "input": "",
        "ok": true
      },
      {
        "input": "<root/>",
        "output": "{\"root\":\"\"}",
        "ok": true
      },
      {
        "input": "<root></root>",
        "output": "{\"root\":\"\"}",
        "ok": true
      }
    ]
  },
  "sample_response": {
    "pre_transform": "<Response><DescribeVolumesResponse><NextToken>sample_string</NextToken><Volumes><item><AvailabilityZone>sample_string</AvailabilityZone><Attachments><item></item></Attachments><Size>0</Size><State>sample_string</State><FastRestored>false</FastRestored><CreateTime>sample_string</CreateTime><SnapshotId>sample_string</SnapshotId><Encrypted>false</Encrypted><Iops>0</Iops><Throughput>0</Throughput><KmsKeyId>sample_string</KmsKeyId><OutpostArn>sample_string</OutpostArn><VolumeType>sample_string</VolumeType><Tags><item></item></Tags><VolumeId>sample_string</VolumeId><MultiAttachEnabled>false</MultiAttachEnabled></item></Volumes></DescribeVolumesResponse></Response>",
    "post_transform": "{\n  \"DescribeVolumesResponse\": {\n    \"NextToken\": \"sample_string\",\n    \"Volumes\": [\n      {\n        \"Attachments\": [\n          {}\n        ],\n        \"AvailabilityZone\": \"sample_string\",\n        \"CreateTime\": \"sample_string\",\n        \"Encrypted\": false,\n        \"FastRestored\": false,\n        \"Iops\": 0,\n        \"KmsKeyId\": \"sample_string\",\n        \"MultiAttachEnabled\": false,\n        \"OutpostArn\": \"sample_string\",\n        \"Size\": 0,\n        \"SnapshotId\": \"sample_string\",\n        \"State\": \"sample_string\",\n        \"Tags\": [\n          {}\n        ],\n        \"Throughput\": 0,\n        \"VolumeId\": \"sample_string\",\n        \"VolumeType\": \"sample_string\"\n      }\n    ]\n  }\n}"
  },
  "mock_route": "@app.route('/', methods=['POST'])\ndef aws_ec2_volumes_post_naively_presented_describevolumes():\n    if request.form.get('Action') == 'DescribeVolumes':\n        return Response(MOCK_RESPONSE_AWS_EC2_VOLUMES_POST_NAIVELY_PRESENTED_DESCRIBEVOLUMES, content_type='application/xml')",
  "stackql_query": "SELECT * FROM aws.ec2.volumes_post_naively_presented WHERE region = 'dummy_region'",
  "expected_response": "[\n  {\n    \"DescribeVolumesResponse\": {\n      \"NextToken\": \"sample_string\",\n      \"Volumes\": [\n        {\n          \"Attachments\": [\n            {}\n          ],\n          \"AvailabilityZone\": \"sample_string\",\n          \"CreateTime\": \"sample_string\",\n          \"Encrypted\": false,\n          \"FastRestored\": false,\n          \"Iops\": 0,\n          \"KmsKeyId\": \"sample_string\",\n          \"MultiAttachEnabled\": false,\n          \"OutpostArn\": \"sample_string\",\n          \"Size\": 0,\n          \"SnapshotId\": \"sample_string\",\n          \"State\": \"sample_string\",\n          \"Tags\": [\n            {}\n          ],\n          \"Throughput\": 0,\n          \"VolumeId\": \"sample_string\",\n          \"VolumeType\": \"sample_string\"\n        }\n      ]\n    }\n  }\n]"
}
```