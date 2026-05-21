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
    restish_api_name: str | None
    project_id: str | None

    def __init__(self, config_path: Path):
        self.config_path = config_path
        try:
            with open(config_path, 'r') as f:
                config = json.load(f)
            self.restish_api_name = config.get("restish_api", None)
            self.project_id = config.get("project_id", None)

        except FileNotFoundError:
            self.restish_api_name = None
            self.project_id = None

    def save(self):
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(self.config_path, 'w') as f:
            content = {
                'restish_api': self.restish_api_name,
                'project_id': self.project_id
            }
            json.dump(content, f)

    def restish(self, *args) -> str:
        assert self.restish_api_name, "restish API name not configured"
        assert self.project_id, "project ID not configured"
        return restish(self.restish_api_name, *args, "-H", "apimetrics-project-id:" + self.project_id)


def restish(api_name: str, *args: str) -> str:
    """
    Invoke restish with the given arguments.
    :param api_name: the name of the restish api to invoke
    :param args: arguments to pass to restish
    :return: the output of restish
    """
    print(f"restish {api_name} {' '.join(args)}")
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


def configure_restish(config: Config):
    if not config.restish_api_name:
        config.restish_api_name = RESTISH_API_NAME
        config.save()

    apis = load_restish_apis()
    if config.restish_api_name in apis:
        # already configured
        return

    apis[config.restish_api_name] = {
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


def select_project(config: Config) -> str:
    """
    List the projects and prompt the user to select one.
    :return: The id of the selected project.
    """
    response = json.loads(config.restish("account-list-projects"))
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
    configure_restish(config)

    if not config.project_id:
        config.project_id = select_project(config)
        config.save()

    if not config.project_id:
        print("No project selected. Exiting.")
        exit(1)

    args = sys.argv[1:]
    if len(args):
        print(config.restish(*args))
