from flask import Flask, request, Response

app = Flask(__name__)

MOCK_RESPONSE_AWS_EC2_INSTANCES_DESCRIBE = """<Response><line_items><item><instance_type>sample_string</instance_type><iam_instance_profile></iam_instance_profile><instance_id>sample_string</instance_id><public_ip_address>sample_string</public_ip_address><private_dns_name>sample_string</private_dns_name><launch_time>sample_string</launch_time><private_ip_address>sample_string</private_ip_address><key_name>sample_string</key_name><state></state><public_dns_name>sample_string</public_dns_name><security_groups></security_groups><monitoring></monitoring><availability_zone>sample_string</availability_zone><image_id>sample_string</image_id><tag_set></tag_set><network_interfaces></network_interfaces><subnet_id>sample_string</subnet_id><vpc_id>sample_string</vpc_id><block_device_mappings></block_device_mappings></item></line_items><next_page_token>sample_string</next_page_token></Response>"""

@app.route('/', methods=['POST'])
def aws_ec2_instances_describe():
    return Response(MOCK_RESPONSE_AWS_EC2_INSTANCES_DESCRIBE, content_type='application/json')


if __name__ == '__main__':
    import argparse as ap
    p = ap.ArgumentParser()
    p.add_argument('--port', type=int, default=5000)
    app.run(host='0.0.0.0', port=p.parse_args().port)
