
# Generic TCP/IP support

If we model provider systems as interfaces addressable on the internet,
then `any-sdk` ought not be restricted to HTTP(S).  The motivating use cases include:

- LDAP.  Can be developed with [this freebie container](https://hub.docker.com/r/bitnami/openldap).  Business case to support AD and other directories is strong.
- Various RDBMS protocols.
- TELNET.  Why not?  [This stackoverflow answer](https://stackoverflow.com/questions/15772355/how-to-send-an-http-request-using-telnet) is a nice, simple example.
- DHCP.
- DNS.
- ICMP.

There is no fundamental reason we cannot also address unix sockets.

