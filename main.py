from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
import functools
import json
import os
import re
import subprocess
import sys

APIMETRICS_BASE_URL = os.environ.get("APIMETRICS_BASE_URL", "https://qc-client.apimetrics.io")
APIMETRICS_AUTH_URL = os.environ.get("APIMETRICS_AUTH_URL", "https://qc-auth.apimetrics.io/authorize")
APIMETRICS_TOKEN_URL = os.environ.get("APIMETRICS_TOKEN_URL", "https://qc-auth.apimetrics.io/oauth/token")
APIMETRICS_AUTH_AUDIENCE = os.environ.get("APIMETRICS_AUTH_AUDIENCE", "https://client.apimetrics.io")
APIMETRICS_CLIENT_ID = os.environ.get("APIMETRICS_CLIENT_ID", "bj0yh0AjBMzfeOpffmCj5UP8FbmYDwcM")
RESTISH_API_NAME = os.environ.get("RESTISH_API_NAME", "apimetrics")


@dataclass
class Config:
    config_path: Path
    restish_configured: bool
    authenticated: bool
    project_id: str | None

    def __init__(self, config_path: Path):
        self.config_path = config_path
        try:
            with open(config_path, 'r') as f:
                config = json.load(f)
            self.restish_configured = config.get('restish_configured', False)
            self.project_id = config.get('project_id', None)

        except FileNotFoundError:
            self.restish_configured = False
            self.project_id = None

    def save(self):
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(self.config_path, 'w') as f:
            content = {
                'restish_configured': self.restish_configured,
                'project_id': self.project_id
            }
            json.dump(content, f)


def restish(*args) -> str:
    """
    Invoke restish with the given arguments.
    :param args: arguments to pass to restish
    :return: the output of restish
    """
    print(f"restish {RESTISH_API_NAME} {' '.join(args)}")
    proc = subprocess.run(
        ["restish", RESTISH_API_NAME, *args],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT
    )
    return proc.stdout


@functools.cache
def get_restish_config_path() -> Path:
    """
    Find the path to the restish config directory.
    :return: A Path object pointing to the restish config directory.
    """
    output = restish("localhost", "-v")
    config_re = re.compile(r".*Configuration: map\[(.*)\]$")
    keyword_re = re.compile(r"([\w\-]+):")
    for line in output.splitlines():
        match = config_re.match(line)
        if match:
            params = re.split(keyword_re, match.group(1))
            config_dir = params.index("config-directory")
            if config_dir != -1:
                return Path(params[config_dir + 1].strip())
    raise ValueError("Could not find config directory in restish output")


def load_restish_apis() -> dict:
    """
    Load the apis.json file from the restish config directory.
    :return: A dictionary containing the apis.json content
    """
    config_path = get_restish_config_path()
    try:
        with open(config_path / "apis.json", "rt") as f:
            return dict(json.load(f))
    except FileNotFoundError:
        return { "$schema": "https://rest.sh/schemas/apis.json" }


def save_restish_apis(apis: dict):
    config_path = get_restish_config_path()
    config_path.mkdir(parents=True, exist_ok=True)
    with open(config_path / "apis.json", "wt") as f:
        json.dump(apis, f, indent=2)


def configure_restish():
    apis = load_restish_apis()
    apis["apimetrics"] = {
        "base": APIMETRICS_BASE_URL,
        "profiles": {
            "default": {
                "auth": {
                    "name": "oauth-authorization-code",
                    "params": {
                        "authorize_url": f"{APIMETRICS_AUTH_URL}?audience={APIMETRICS_AUTH_AUDIENCE}",
                        "token_url": APIMETRICS_TOKEN_URL,
                        "client_id": APIMETRICS_CLIENT_ID,
                        "client_secret": "",
                        "redirect_url": "",
                        "scopes": "openid profile email",
                    }
                }
            }
        }
    }
    save_restish_apis(apis)


def select_project():
    response = json.loads(restish("account-list-projects"))
    projects = response["projects"]
    projects_by_org = defaultdict(list)
    for project in projects:
        projects_by_org[project["project"]["org_id"]].append(project)

    if len(projects_by_org) > 1:
        orgs = response["organizations"]
        print("Select an organization:")
        org_ids = sorted(projects_by_org.keys(), key=lambda x: orgs.get(x, {}).get("name", ""))
        for i, org_id in enumerate(org_ids):
            org_name = "Personal Projects" if not org_id else orgs[org_id]["name"]
            print(f"{i}: {org_name}")
        org_index = int(input("Enter the number of the organization you want to use: "))
        projects = projects_by_org[org_ids[org_index]]

    projects = sorted(projects, key=lambda x: x["project"]["name"])
    print("Select a project:")
    for i, project in enumerate(projects):
        print(f"{i}: {project["project"]["name"]}")
    project_index = int(input("Enter the number of the project you want to use: "))
    return projects[project_index]['id']


if __name__ == '__main__':
    config_path = Path.home() / ".config" / "apimetrics" / "config.json"
    config = Config(config_path)
    if not config.restish_configured:
        configure_restish()
        config.restish_configured = True
        config.save()

    if not config.project_id:
        config.project_id = select_project()
        if not config.project_id:
            print("No project selected. Exiting.")
            exit(1)
        config.save()

    args = sys.argv[1:]
    if len(args):
        print(restish("-H", f"apimetrics-project-id:{config.project_id}", *args))
