# StackQL Provider Specification

This document provides a comprehensive specification for StackQL providers, which are OpenAPI 3.0 documents augmented with custom `x-stackQL-*` extensions that enable SQL-like semantics for REST APIs.

## Table of Contents

1. [Overview](#overview)
2. [Provider Hierarchy](#provider-hierarchy)
3. [Provider Document Structure](#provider-document-structure)
4. [Service Document Structure](#service-document-structure)
5. [Resource Definition](#resource-definition)
6. [Method Definition](#method-definition)
7. [SQL Verb Mapping](#sql-verb-mapping)
8. [Configuration Options](#configuration-options)
9. [Configuration Inheritance](#configuration-inheritance)
10. [OpenAPI Extensions Reference](#openapi-extensions-reference)
11. [Response Processing](#response-processing)
12. [Authentication](#authentication)
13. [Pagination](#pagination)

---

## Overview

StackQL providers transform REST APIs into SQL-queryable resources. A provider consists of:

- **Provider Document** (`provider.yaml`): Root configuration containing provider metadata and service references
- **Service Documents**: OpenAPI 3.0 specifications with StackQL extensions defining resources and methods
- **Resources**: Logical groupings of related API operations mapped to SQL tables
- **Methods**: Individual API operations mapped to SQL verbs (SELECT, INSERT, UPDATE, DELETE)

---

## Provider Hierarchy

```
Provider
├── config (provider-level)
└── ProviderServices
    ├── config (service-level)
    └── Service (OpenAPI document)
        └── x-stackQL-resources
            └── Resource
                ├── config (resource-level)
                └── Methods
                    └── Method
                        └── config (method-level)
```

Configuration cascades from higher levels to lower levels, with lower-level config overriding higher-level config for most options. See [Configuration Inheritance](#configuration-inheritance) for details.

---

## Provider Document Structure

The provider document (`provider.yaml`) is the entry point for a StackQL provider.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique provider identifier |
| `name` | string | Provider name (e.g., "google", "aws", "azure") |
| `title` | string | Human-readable title |
| `version` | string | Provider version |
| `providerServices` | map | Map of service names to service definitions |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Provider description |
| `protocolType` | string | Protocol type: `"http"` (default) or `"local_templated"` |
| `config` | object | Provider-level configuration |
| `responseKeys` | object | Default response extraction keys |

### Example

```yaml
id: example
name: example
title: Example Provider
version: v1.0.0
description: Example StackQL provider
protocolType: http

providerServices:
  api:
    id: example.api
    name: api
    title: Example API
    version: v1
    preferred: true
    service:
      $ref: services/api.yaml
    resources:
      $ref: resources/api-resources.yaml

config:
  auth:
    type: api_key
    name: X-API-Key
    location: header
    api_key_var: EXAMPLE_API_KEY
  pagination:
    requestToken:
      key: page
      location: query
    responseToken:
      key: nextPage
      location: body

responseKeys:
  selectItemsKey: "items"
  deleteItemsKey: "id"
```

---

## Service Document Structure

Service documents are OpenAPI 3.0 specifications with StackQL extensions.

### Structure

```yaml
openapi: 3.0.0
info:
  title: Service Title
  version: "1.0.0"

servers:
  - url: https://api.example.com
    variables:
      region:
        default: us-east-1
        enum: [us-east-1, us-west-2]

components:
  x-stackQL-resources:
    # Resource definitions (see Resource Definition section)

  schemas:
    # OpenAPI schemas

  parameters:
    # Reusable parameters

  securitySchemes:
    # Authentication schemes

paths:
  # OpenAPI path definitions

x-stackQL-config:
  # Service-level configuration (optional)
```

---

## Resource Definition

Resources are defined within `components.x-stackQL-resources` and represent SQL-queryable tables.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Fully qualified resource ID (e.g., `provider.service.resource`) |
| `name` | string | Resource name (typically snake_case, plural) |
| `methods` | map | Map of method names to method definitions |
| `sqlVerbs` | object | SQL verb mappings |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Human-readable title |
| `description` | string | Resource description |
| `config` | object | Resource-level configuration |
| `serviceDoc` | object | Reference to service document |
| `selectorAlgorithm` | string | Method selection algorithm |
| `baseUrl` | string | Base URL override |

### Example

```yaml
components:
  x-stackQL-resources:
    instances:
      id: google.compute.instances
      name: instances
      title: Compute Instances
      description: Virtual machine instances

      methods:
        list:
          operation:
            $ref: '#/paths/~1projects~1{project}~1zones~1{zone}~1instances/get'
          response:
            mediaType: application/json
            openAPIDocKey: '200'
            objectKey: $.items
        get:
          operation:
            $ref: '#/paths/~1projects~1{project}~1zones~1{zone}~1instances~1{instance}/get'
          response:
            mediaType: application/json
            openAPIDocKey: '200'
        insert:
          operation:
            $ref: '#/paths/~1projects~1{project}~1zones~1{zone}~1instances/post'
          request:
            mediaType: application/json
          response:
            mediaType: application/json
            openAPIDocKey: '200'
        delete:
          operation:
            $ref: '#/paths/~1projects~1{project}~1zones~1{zone}~1instances~1{instance}/delete'
          response:
            mediaType: application/json
            openAPIDocKey: '200'

      sqlVerbs:
        select:
          - $ref: '#/components/x-stackQL-resources/instances/methods/list'
          - $ref: '#/components/x-stackQL-resources/instances/methods/get'
        insert:
          - $ref: '#/components/x-stackQL-resources/instances/methods/insert'
        update: []
        replace: []
        delete:
          - $ref: '#/components/x-stackQL-resources/instances/methods/delete'
```

---

## Method Definition

Methods define individual API operations and how they map to SQL semantics.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `operation` | object | Reference to OpenAPI operation (`$ref`) |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `response` | object | Response configuration |
| `request` | object | Request configuration |
| `config` | object | Method-level configuration |
| `servers` | array | Operation-specific servers |
| `inverse` | object | Rollback operation definition |
| `apiMethod` | string | HTTP method override |
| `serviceName` | string | Service name override |

### Response Configuration

```yaml
response:
  mediaType: application/json           # Response content type
  openAPIDocKey: '200'                  # Response code to use for schema
  objectKey: $.items                    # JSONPath to extract items
  overrideMediaType: application/json   # Override response parsing
  schema_override:                      # Custom response schema
    $ref: '#/components/schemas/CustomSchema'
  async_schema_override:                # Async operation schema
    $ref: '#/components/schemas/AsyncSchema'
  asyncOverrideMediaType: application/json
  projection_map:                       # Response field projections
    alias: actualFieldName
  transform:                            # Response transformation
    type: golang_template_mxj_v0.1.0
    body: |
      {"processed": {{.field}}}
```

### Request Configuration

```yaml
request:
  mediaType: application/json           # Request content type
  default: '{"key": "default_value"}'   # Default request body
  base: '{"always": "included"}'        # Base request body (always merged)
  required: [field1, field2]            # Required body fields
  projection_map:                       # Request field projections
    alias: actualFieldName
  xmlDeclaration: '<?xml version="1.0"?>'
  xmlTransform: unescape
  xmlRootAnnotation: '<root xmlns="...">'
```

### Complete Example

```yaml
methods:
  create:
    operation:
      $ref: '#/paths/~1resources/post'

    request:
      mediaType: application/json
      required: [name, type]

    response:
      mediaType: application/json
      openAPIDocKey: '201'

    config:
      queryParamPushdown:
        filter:
          dialect: odata
          supportedOperators: ["eq", "ne"]

    inverse:
      sqlVerb:
        $ref: '#/components/x-stackQL-resources/resources/methods/delete'
      tokens:
        resourceId:
          key: $.id
          location: body
          algorithm: jsonpath
```

---

## SQL Verb Mapping

SQL verbs map SQL query types to API methods.

### Supported Verbs

| SQL Verb | SQL Statement | Typical HTTP Method |
|----------|--------------|---------------------|
| `select` | SELECT | GET |
| `insert` | INSERT | POST |
| `update` | UPDATE | PATCH |
| `replace` | REPLACE (full update) | PUT |
| `delete` | DELETE | DELETE |
| `exec` | EXEC (procedures) | Any |

### Method Selection Algorithm

When executing a SQL query, StackQL selects the appropriate method based on:

1. **SQL Verb Match**: Match query type to `sqlVerbs` mapping
2. **Parameter Match**: Find methods where provided parameters satisfy required parameters
3. **Selectivity Order**: Methods are tried in order listed; place less selective methods (fewer required params) first

### Overloading Example

```yaml
sqlVerbs:
  select:
    # List method - no required path params, matches broad queries
    - $ref: '#/components/x-stackQL-resources/instances/methods/list'
    # Get method - requires instance ID, matches specific queries
    - $ref: '#/components/x-stackQL-resources/instances/methods/get'
```

Query behavior:
- `SELECT * FROM instances WHERE project='p' AND zone='z'` → Uses `list` method
- `SELECT * FROM instances WHERE project='p' AND zone='z' AND instance='i'` → Uses `get` method

---

## Configuration Options

Configuration can be set at provider, providerService, resource, or method levels.

### Available Config Options

| Option | Description | Inherits? |
|--------|-------------|-----------|
| `auth` | Authentication configuration | Yes |
| `pagination` | Pagination token handling | Yes |
| `queryParamTranspose` | Query parameter transformation | Yes |
| `requestTranslate` | Request transformation | Yes |
| `requestBodyTranslate` | Request body transformation | Yes |
| `variations` | Schema variation handling | Yes |
| `views` | SQL view definitions | No |
| `sqlExternalTables` | External table definitions | No |
| `queryParamPushdown` | Query pushdown to API params | **No** |

### Config Structure

```yaml
config:
  auth:
    type: api_key
    name: X-API-Key
    location: header

  pagination:
    requestToken:
      key: pageToken
      location: query
    responseToken:
      key: nextPageToken
      location: body

  queryParamTranspose:
    algorithm: default

  requestTranslate:
    algorithm: default

  requestBodyTranslate:
    algorithm: naive

  variations:
    isObjectSchemaImplicitlyUnioned: false

  views:
    select:
      predicate: 'sqlDialect == "stackql"'
      ddl: |
        SELECT id, name FROM resource

  sqlExternalTables:
    external_data:
      catalogName: external
      schemaName: public
      name: data
      columns:
        - name: id
          type: string

  queryParamPushdown:
    filter:
      dialect: odata
      supportedOperators: ["eq", "ne"]
```

---

## Configuration Inheritance

Most configuration options inherit from higher levels in the hierarchy:

```
Method → Resource → Service → ProviderService → Provider
```

**With Inheritance** (lower level overrides higher level):
- `auth`
- `pagination`
- `queryParamTranspose`
- `requestTranslate`
- `requestBodyTranslate`
- `variations`

**Without Inheritance** (must be set at specific level):
- `queryParamPushdown` - **Must be set at METHOD level only**
- `views` - Set at method or resource level
- `sqlExternalTables` - Set at method or resource level

### Query Parameter Pushdown Location

The `queryParamPushdown` configuration must be placed within a method's `config` block:

```yaml
components:
  x-stackQL-resources:
    people:
      id: provider.service.people
      name: people
      methods:
        list:
          operation:
            $ref: '#/paths/~1people/get'
          response:
            mediaType: application/json
            openAPIDocKey: '200'
            objectKey: $.value[*]
          config:                          # Method-level config
            queryParamPushdown:            # Must be here, not at resource level
              filter:
                dialect: odata
                supportedOperators: ["eq", "ne", "contains"]
              select:
                dialect: odata
              orderBy:
                dialect: odata
              top:
                dialect: odata
      sqlVerbs:
        select:
          - $ref: '#/components/x-stackQL-resources/people/methods/list'
```

---

## OpenAPI Extensions Reference

### Document-Level Extensions

| Extension | Location | Description |
|-----------|----------|-------------|
| `x-stackQL-resources` | `components` | Resource definitions |
| `x-stackQL-config` | Root or `components` | Service-level configuration |
| `x-stackQL-provider` | `info` | Embedded provider metadata |

### Operation-Level Extensions

| Extension | Description |
|-----------|-------------|
| `x-stackQL-resource` | Associates operation with a resource |
| `x-stackQL-method` | Specifies method name |
| `x-stackQL-verb` | Maps to SQL verb |
| `x-stackQL-objectKey` | JSONPath for response extraction |
| `x-stackQL-graphQL` | GraphQL operation configuration |

### Schema-Level Extensions

| Extension | Description |
|-----------|-------------|
| `x-stackQL-stringOnly` | Serialize as string regardless of type |
| `x-stackQL-alias` | Property aliasing |
| `x-alwaysRequired` | Mark parameter as always required |

---

## Response Processing

### Object Key Extraction

The `objectKey` field specifies how to extract items from API responses.

**JSONPath Examples:**
```yaml
objectKey: $.items              # Array at 'items' key
objectKey: $.data.results       # Nested path
objectKey: $[*]                 # Root array
objectKey: $.value[*]           # OData-style response
```

**XPath Examples (for XML):**
```yaml
objectKey: /Response/Items/Item
objectKey: //Volume
```

### Response Transformation

Transform responses using Go templates:

```yaml
response:
  transform:
    type: golang_template_mxj_v0.1.0
    body: |
      {
        "items": [
          {{- range $i, $item := .data.items }}
          {{- if $i}},{{end}}
          {"id": {{printf "%q" $item.id}}, "name": {{printf "%q" $item.name}}}
          {{- end }}
        ]
      }
```

---

## Authentication

### Supported Auth Types

| Type | Description |
|------|-------------|
| `api_key` | API key in header or query |
| `basic` | HTTP Basic authentication |
| `bearer` | Bearer token |
| `service_account` | Google-style service account |
| `oauth2` | OAuth 2.0 client credentials |
| `aws_signing_v4` | AWS Signature Version 4 |
| `azure_default` | Azure default credentials |
| `custom` | Custom authentication |

### Example Configurations

**API Key:**
```yaml
auth:
  type: api_key
  name: X-API-Key
  location: header           # or "query"
  api_key_var: MY_API_KEY    # Environment variable
```

**OAuth2 Client Credentials:**
```yaml
auth:
  type: oauth2
  grant_type: client_credentials
  token_url: https://auth.example.com/token
  client_id_env_var: CLIENT_ID
  client_secret_env_var: CLIENT_SECRET
  scopes:
    - read
    - write
```

**Service Account (Google):**
```yaml
auth:
  type: service_account
  credentialsenvvar: GOOGLE_CREDENTIALS
  scopes:
    - https://www.googleapis.com/auth/cloud-platform
```

---

## Pagination

### Token-Based Pagination

```yaml
pagination:
  requestToken:
    key: pageToken           # Request parameter name
    location: query          # "query", "header", or "body"
  responseToken:
    key: nextPageToken       # Response field with next token
    location: body           # "body" or "header"
```

### Offset-Based Pagination

```yaml
pagination:
  requestToken:
    key: offset
    location: query
  responseToken:
    key: next_offset
    location: body
```

### Link Header Pagination (GitHub-style)

```yaml
pagination:
  requestToken:
    key: page
    location: query
  responseToken:
    key: Link
    location: header
    algorithm: link_header_next
```

---

## Query Parameter Pushdown

Enables SQL clause pushdown to API query parameters for filtering, projection, ordering, and limiting.

### Supported Pushdown Types

| Type | SQL Clause | OData Parameter | Description |
|------|------------|-----------------|-------------|
| `filter` | WHERE | `$filter` | Row filtering |
| `select` | SELECT columns | `$select` | Column projection |
| `orderBy` | ORDER BY | `$orderby` | Result ordering |
| `top` | LIMIT | `$top` | Row limiting |
| `count` | COUNT(*) | `$count` | Total count |

### OData Configuration

```yaml
config:
  queryParamPushdown:
    select:
      dialect: odata
    filter:
      dialect: odata
      supportedOperators:
        - "eq"
        - "ne"
        - "gt"
        - "lt"
        - "ge"
        - "le"
        - "contains"
        - "startswith"
        - "endswith"
    orderBy:
      dialect: odata
    top:
      dialect: odata
    count:
      dialect: odata
```

### Custom API Configuration

```yaml
config:
  queryParamPushdown:
    select:
      paramName: "fields"
      delimiter: ","
    filter:
      paramName: "filter"
      syntax: "key_value"
      supportedOperators: ["eq"]
      supportedColumns: ["status", "region"]
    orderBy:
      paramName: "sort"
      syntax: "prefix"
      supportedColumns: ["createdAt", "name"]
    top:
      paramName: "limit"
      maxValue: 100
```

### Supported Filter Syntaxes

| Syntax | Example Output |
|--------|---------------|
| `odata` | `$filter=status eq 'active'` |
| `key_value` | `filter[status]=active` |
| `simple` | `status=active` |

### Supported OrderBy Syntaxes

| Syntax | Example |
|--------|---------|
| `odata` | `$orderby=name desc` |
| `prefix` | `sort=-name` |
| `suffix` | `sort=name:desc` |

---

## Best Practices

1. **Resource Naming**: Use plural, snake_case names (e.g., `instances`, `storage_accounts`)

2. **Method Naming**: Use descriptive names matching the operation (`list`, `get`, `create`, `delete`)

3. **SQL Verb Ordering**: Order methods by ascending parameter count for correct selection

4. **Object Keys**: Use JSONPath for consistent response extraction

5. **Schema Reuse**: Define schemas in `components/schemas` and reference them

6. **Configuration Placement**:
   - Place common config (auth, pagination) at provider or service level
   - Place `queryParamPushdown` at method level

7. **Parameter Validation**: Use OpenAPI schema validation for parameters

8. **Documentation**: Include descriptions for resources, methods, and parameters
