<!-- language: lang-none -->

![Platforms](https://img.shields.io/badge/platform-windows%20macos%20linux-brightgreen)
![Go](https://github.com/stackql/stackql/workflows/Go/badge.svg)
![License](https://img.shields.io/github/license/stackql/stackql)
![Lines](https://img.shields.io/tokei/lines/github/stackql/stackql)   


# any-sdk

A golang library to support:
  - traversal algorithms on StackQL augmented openapi doc structure.
  - SQL semantics thereupon.

From those who brought you

[![StackQL](https://stackql.io/img/stackql-banner.png)](https://stackql.io/)

## Evolution to protocol agnostic

The basic idea is a full decouple and abstraction of the interface from openapi.

Based upon the fact that [golang text templates are Turing complete](https://linuxtut.com/en/2072207ec0565a80d2b2/), as are numerous other DSLs, we can use these to define and route SQL input to arbitrary interfaces.  For instance, here is [the brainf@$& interpreter](https://github.com/Syuparn/go-template-bf-interpreter/blob/1b7f6a3720295c93ffa99b58a81f153bd8d7ecc8/bf-interpreter.tpl) described in the article.

## Acknowledgements

Extensions, adaptations and shims of the following support our work:

  - [kin-openapi](https://github.com/getkin/kin-openapi)
  - [gorilla/mux](https://github.com/gorilla/mux)

We gratefully acknowledge these pieces of work.

## Licensing

Please see the [stackql LICENSE](/LICENSE).

Licenses for third party software we are using are included in the [/licenses](/licenses) directory.
