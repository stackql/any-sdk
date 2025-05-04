from asyncio import subprocess
import json
import os
import time
import typing

from robot.api.deco import keyword, library
from robot.libraries.BuiltIn import BuiltIn
from robot.libraries.Collections import Collections
from robot.libraries.Process import Process
from robot.libraries.OperatingSystem import OperatingSystem 

from .registry_cfg import RegistryCfg
from .ShellSession import ShellSession
from .psycopg_client import PsycoPGClient
from .psycopg2_client import PsycoPG2Client
from .sqlalchemy_client import SQLAlchemyClient

SQL_BACKEND_CANONICAL_SQLITE_EMBEDDED :str = 'sqlite_embedded'
SQL_BACKEND_POSTGRES_TCP :str = 'postgres_tcp'
SQL_CONCURRENCT_LIMIT_DEFAULT :int = 1

_TEST_APP_CACHE_ROOT = os.path.join("test", ".stackql")

PSQL_EXE :str = os.environ.get('PSQL_EXE', 'psql')
SQLITE_EXE :str = os.environ.get('SQLITE_EXE', 'sqlite3')


@library(scope='SUITE', version='0.1.0', doc_format='reST')
class any_sdk_cli_testing(OperatingSystem, Process, BuiltIn, Collections):
  ROBOT_LISTENER_API_VERSION = 2

  def __init__(self, execution_platform='native', sql_backend=SQL_BACKEND_CANONICAL_SQLITE_EMBEDDED, concurrency_limit=SQL_CONCURRENCT_LIMIT_DEFAULT):
    self._counter = 0
    self._execution_platform=execution_platform
    self._sql_backend=sql_backend
    self._concurrency_limit=concurrency_limit
    self.ROBOT_LIBRARY_LISTENER = self
    Process.__init__(self)

  @keyword
  def should_any_sdk_cli_inline_equal_both_streams(
    self, 
    stackql_exe :str, 
    okta_secret_str :str,
    github_secret_str :str,
    k8s_secret_str :str,
    registry_cfg :RegistryCfg, 
    auth_cfg_str :str, 
    sql_backend_cfg_str :str,
    query :str,
    expected_output :str,
    expected_stderr_output :str,
    *args,
    **cfg
  ):
    repeat_count = int(cfg.pop('repeat_count', 1))
    for _ in range(repeat_count):
      result = self._run_stackql_exec_command(
        stackql_exe, 
        okta_secret_str,
        github_secret_str,
        k8s_secret_str,
        registry_cfg, 
        auth_cfg_str, 
        sql_backend_cfg_str,
        query,
        *args,
        **cfg
      )
      return self._verify_both_streams(result, expected_output, expected_stderr_output)