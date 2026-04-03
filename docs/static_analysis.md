
# Static Analysis

This library contains normative static analysis tooling for provider documents.  The cli exposes this at different granularities.


## Method closures

For any collection of `stackql` methods (those referenced under `components.x-stackQL-resources.<resource name>.sqlVerbs` or implicityl defaulted to the "SQL" semantic `EXEC` under `components.x-stackQL-resources.<resource name>.methods`), there exists a `closure` object graph and corresponding document which is:

- Necessary and sufficient to present and action these methods.
- A subset of the original document.

There should exist a static analysis subcomponent that supports:

- Derivation of closures for method collections. 
- Serialization of closures into document form.
- Optional rewrite of aspects.  
    - Initially let us not be too sophisticated, scheme, host, port rewrite is the main initial use case.
    - Some object that encapsulates rewriting semantics might be good so we can extend later.
- Also a capability for a workflow where whole documents or provider doc collections and ingested, rewritten and then serialized.
- This functionality should be available through the cli.






