
# Generic TCP/IP support

If we model provider systems as interfaces addressable on the internet,
then `any-sdk` ought not be restricted to HTTP(S).  The motivating use cases include:

- LDAP.  Can be developed with [this freebie container](https://hub.docker.com/r/bitnami/openldap).  Business case to support AD and other directories is strong.
- Various RDBMS protocols.
- TELNET.  Why not?
- DHCP.
- DNS.
- ICMP.

There is no fundamental we cannot also address unix sockets.

