

# Aliasing

## In the abstract


Any query's address space is composed of:

1. Zero or one URLs (eg https://host:port/path/to/resource?queryParam1=a&queryParam2=b).
2. Input data on some logical "page" eg: an HTTP request.  Another example: a golang template and fuinctionality to call a CLI tool.
3. Returned data on some logical "page", eg: an HTTP response.

Pages can in theory be collections of pages and populated with a mixture of static and dynamic data.  The namespace is a search structure to locate any address.  

> For an arbitrary symbol in a query flow, a namespace is some system that traverses the entire address space at arbitrary depth and finds the appropriate location.

Updates through this approach will be staged through some shadow data structure(s).

`openapi` provides some, but certainly not all, of this functionality.    Putting aside the implementation, this is possible so long as page collections are named or strictly ordered.  

### Desired Namespace

For SQL semantics, a flat namespace without arbitrary prefixes and some semantic relevance is preferred.  This suggests a configurable aliasing capability, coupled with a collision resolution algorithm.  The aliasing capability:

- For input data, must support transparent rewrite w.r.t. the provider system.  Ie: the provider receives precisely the unaliased data.  This implies that any existing rewrite logic must either be ignored or be consistent with the aliasing.
- For output data, simply a lazy rewrite before staging in RDBMS / relational algebra engine is ok.

Something like:

- Flatten the address space based upon config and default behaviour.  There is already a simply version of this in `objectKey`.
- Search the flattened address space based on cofiguration and default behaviour, identifying conflicts.
- For each conflict, apply a configurable resolution algorithm.
- At runtime, perform the aliasing transform relations and display data accordingly.



## For HTTP


A useful consideration to begin with is the native capabilities of `openapi`, which supports dynamic substitution of URLs and requests for:

- Named [`parameter` objects](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#parameter-object) in "query", "header", "path" or "cookie" aspects.  These are unique in the tuple (`name`, `location`), and map to one or more locations in the query address space.  For an example of plurality: query parameters may be repeated per [RFC 3986](https://datatracker.ietf.org/doc/html/rfc3986).
- Named [server `variable` objects](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#server-variable-object) in server URLs.


Request and response **sub-component** (body, header) attributes can be exposed unqualified, provided they do not collide with other exposed attributes.  Such collisions are frequent and so naive and sub-optimal measures are available:

- Not exposing response header attributes at all.
- Heuristics that effectively clobber any input attribute that collides with a server variable.
- Routing request body attributes based on a prefix eg: `data__`.  This is undesirable on the grounds of poor encapsulation and weakened semantics.
  - Interesting schema search implementation in `getSelectItemsSchema()`.

### Proposed HTTP implementation

Requirements:

- Aliased `openapi` parameters to include an alias extension attribute.
- Some config exists denoting aliased request and response page attributes.  Call this `PageAliasDirectory`.
- **v2** The existing `objectKey` is enhanced / replaced with a fucnction that supports unions etc.
- A flattening algorithm exists.  Eg: strings -> string, int -> int, object -> string....

Then:

- For all `openapi` parameters, cache alias extension attribute.
- AOT validate flattened namespace.
    - Build namespace search structure.  This will both detect violations and be used downstream, for search and rewrite.
- Runtime perform transform relations.

Search structure:

- Want to be extensible to arbitrary depth.
   - This suggests tree / graph rather than flat map.
   - Some kind of Trie (prefix tree).
- Sits at method level.
   - Makes sense to place aliasing at sql method level, eg sql 

