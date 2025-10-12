# AGENTS.md
### _Operational Invariants and Behavioral Contracts for `any-sdk` Agents_
_Last updated: Draft 0.1_

---

## 1. Purpose

In theory, agents can benefit `any-sdk` by transforming **foreign interface definitions** (e.g., Google Discovery, OpenAPI3, AWS Smithy, Azure SDKs, SaaS REST APIs, or OS procedure call schemas) into normalized **`any-sdk` documents**.  
Such documents drive the functionality of `any-sdk`, enabling agnostic local and internet resource consumption. Once mature, they are stored in [the `stackql-provider-registry` repository](https://github.com/stackql/stackql-provider-registry).  

Both `any-sdk` itself and the document registry are consumed by the application [`stackql`](https://github.com/stackql/stackql), which exposes the resources described as though they were relational database tables — the internet as a database!

The documents consumed by `any-sdk` generally define, per "provider" system, a hierarchy of `provider`, `service`, `resource`, and `methods`. Methods represent interface function calls for data access and mutation. In lieu of strict formalism, we can expand the semantics by example.  

For instance, `google.storage.buckets` has a method named `list`. For this method, defined in `openapi3` grammar, there is a required **query** parameter `project` (an unusual case), which must be supplied by a client for the query to work. For one significant client — [`stackql`](https://github.com/stackql/stackql) — the interfaces exposed by `any-sdk` support SQL semantics.  

Continuing the example, this method can be semantically associated with the SQL verb `SELECT`, and the required `project` parameter must appear in the `WHERE` clause of a `SELECT` against `google.storage.buckets`. There is an informal expectation that all methods explicitly associated with `SELECT` under the same resource will have similar schemas, and it is likely that in future versions, some unioned schema will be published.  

We can illustrate another key concept of `any-sdk` through this example: **method selectivity**.  
When multiple methods under a resource are mapped to the same SQL verb, they should be ordered in the `components.x-stackQL-resources.<resource>.sqlVerbs.<verb>` array by ascending selectivity (i.e., by the number of required parameters).  
Thus, the `list` method should be selected where only `project` is supplied in the `WHERE` clause, while `get` should be selected where both `project` and `bucket` are provided. Syntactically, these associations are JSON pointers to `paths` in the `openapi3` document.

The goal is to produce **semantically correct**, **schema-valid**, and **verifiably grounded** definitions that capture both declarative interface metadata and **implied runtime behaviors** (auth, pagination, errors, rate limits, etc.).

Agents operate under strict invariants and must produce outputs that:

1. Conform to the expected schemas and overall consistency requirements within each provider document collection:
    - The root `provider.yaml` document is a markup description of the provider system itself, containing:
        - A set of required keys.
        - Some significant optional keys. For example, the optional `protocolType` attribute, when set to `local_templated`, indicates that the provider’s constituents are not typical HTTP interfaces but rather OS-level calls whose result streams are transformed using Go text templates.
        - A list of "services" available for the provider.
    - Services and resources are typically grouped together in a single document per service, containing all constituent resources. However, they may be sharded across documents, with cross-document references made using JSON pointers.
        - The core of each service document (for HTTP-exposed services) is defined using `openapi3` grammar, with additional `stackql` semantics expressed in permitted extension attributes.
        - A resource dictionary, canonically located at `components.x-stackQL-resources` in the service document, dictates how resources are located under `provider.service`.

2. Reflect true source semantics.  
3. Pass all automated validation stages.

---

## 2. Contract Overview

### 2.1 Required Output Form
_TBA._  
A formal definition must be supplied.

### 2.2 Prohibited Behaviors
- No hallucinated operations, parameters, or response fields.  
- No unverified default values.  
- No omissions of required invariants (auth, errors, pagination, etc.).  
- No dangling, circular, or ambiguous references.

---

## 3. Global Invariants

### 3.1 Document Schemas

As mentioned above, the root document for each provider is `provider.yaml`; the expected schema is mastered at [`cicd/schema-definitions/provider.schema.json`](/cicd/schema-definitions/provider.schema.json).

For each service document referenced from this root, the schema varies on provider `protocolType`:

- `http` => [`cicd/schema-definitions/service-resources.schema.json`](/cicd/schema-definitions/service-resources.schema.json).
- `local_templated` => [`cicd/schema-definitions/local-templated.service-resources.schema.json`](/cicd/schema-definitions/local-templated.service-resources.schema.json).

There is also support for spliting service and resource files, although this is rarely used.  The schema for such a split out resource file is visible at [`cicd/schema-definitions/fragmented-resources.schema.json`](/cicd/schema-definitions/fragmented-resources.schema.json).

### 3.2 Operation Identity

Each operation must be routable through:

```
<provider>.<service>.<resource>.<method>
```

**Example:**
```
google.compute.instances.list
```

- `service`: canonical service name  
- `version`: stable version identifier  
- `resource`: plural noun (snake_case)  
- `method`: lowercase verb describing intent (`list`, `get`, `create`, `delete`, etc.)

Methods mapped to SQL verbs are annotated in an `sqlVerbs` dictionary, which allows `stackql` to abstract the method layer for agents and users.

---

### 3.3 HTTP Semantics
_TBA._

---

### 3.4 Authentication
_TBA._  
This is yet to be formally defined. OAuth, simple key-based authentication, and various environment-variable-based patterns are supported.  
Unauthenticated systems should be explicitly designated as having `null` auth.

---

### 3.5 Pagination
Opt-in, with default behavior. _TBA._

---

### 3.6 Errors
Canonically aligned with `openapi3`.

---

### 3.7 Rate Limiting
Modeled as a class of failure within a broader collection of possible failure classes, including network and system failures.

---

### 3.8 Long-Running Operations (LRO)
If an operation is asynchronous:
- Different providers have different polling or notification methods.  
- Initially, behavior similar to Google’s `Operation` polling is supported.

---

### 3.9 OS / Non-HTTP Transports
_TBA._  
See the `local_openssl` provider for an example.

---

## 4. Validation Rules

These are informally enforced through the `any-sdk` CLI, specifically the `aot` command.  This includes various configuration options.

---

## 5. Evidence & Traceability
_TBA._

---

### End of Draft 0.1
This draft is intentionally comprehensive to serve as both documentation and a RAG corpus seed.
