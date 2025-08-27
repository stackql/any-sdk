

# Aliasing

## In the abstract


Any query's address space is composed of:

1. Zero or one URLs (eg https://host:port/path/to/resource?queryParam1=a&queryParam2=b).
2. Input data on some logical "page" eg: an HTTP request.  Another example: a golang template and fuinctionality to call a CLI tool.
3. Returned data on some logical "page", eg: an HTTP response.

Pages can in theory be collections of pages and populated with a mixture of static and dynamic data.  The namespace is a search structure to locate any address.  

> For arbitrary address location, some system that traverses the entire address space at arbitrary depth.

`openapi` provides some, but ceertainly not all, of this functionality.    Putting aside the implementation, this is possible so long as page collections are named or strictly ordered.  

### Desired Namespace

For SQL semantics, a flat namespace without arbitrary prefixes and some semantic relevance is preferred.



## For HTTP


A useful consideration to begin with is the native capabilities of `openapi`, which supports dynamic substitution of URLs and requests for:

- Named [`parameter` objects](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#parameter-object) in "query", "header", "path" or "cookie" aspects.  These are unique in the tuple (`name`, `location`), and map to one or more locations in the query address space.  For an example of plurality: query parameters may be repeated per [RFC 3986](https://datatracker.ietf.org/doc/html/rfc3986).
- Named [server `variable` objects](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#server-variable-object) in server URLs.


Request and response **sub-component** (body, header) attributes can be exposed unqualified, provided they do not collide with other exposed attributes.  Such collisions are frequent and so naive measures are available:

- Not exposing response header attributes at all.
- Heuristics that effectively clobber any input attribute that collides with a server variable.
- Routing request body attributes based on a prefix eg: `data__`.

