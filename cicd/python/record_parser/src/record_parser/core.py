import json
import typing

def parse_record(record: str) -> typing.Dict:
    return json.loads(record)

def generate_flask_app(parsed_record: typing.Dict) -> str:
    app_code = f"""from flask import Flask, request, jsonify
app = Flask(__name__)

{parsed_record['sample_response']['var_name']} = "{parsed_record['sample_response']['pre_transform']}"

{parsed_record['mock_route']}

"""
    return app_code


def generate_flask_apps_from_file(file_path: str) -> typing.List[typing.Dict]:
    apps = []
    with open(file_path, 'r') as f:
        for line in f:
            if line.strip():  # Skip empty lines
                parsed_record = parse_record(line)
                if not parsed_record.get('mock_route'):
                    continue  # Skip records without a mock route   
                app_code = generate_flask_app(parsed_record)
                apps.append({
                    "parsed_record": parsed_record,
                    "app_code": app_code
                })
    return apps

def generate_mocks_from_analysis_run(src_dir: str, dest_dir: str):
    import os
    for filename in os.listdir(src_dir):
        if filename.endswith('observations.jsonl'):
            file_path = os.path.join(src_dir, filename)
            apps = generate_flask_apps_from_file(file_path)
            for app in apps:
                mock_filename = f"{app['parsed_record']['provider']}_{app['parsed_record']['service']}_{app['parsed_record']['method']}_mock.py"
                mock_file_path = os.path.join(dest_dir, mock_filename)
                with open(mock_file_path, 'w') as f:
                    f.write(app['app_code'])

