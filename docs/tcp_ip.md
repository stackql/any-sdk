
# Generic TCP/IP support

If we model provider systems as interfaces addressable on the internet,
then `any-sdk` ought not be restricted to HTTP(S).  The motivating use cases include:

- LDAP.  
    - Can be developed with [this freebie container](https://hub.docker.com/r/bitnami/openldap).  There is also a [SAMBA walkthrough](https://avenum.medium.com/how-to-run-an-active-directory-domain-controller-for-free-7037792c8c5a) although latter seems lesser supported. 
    - Business case to support AD and other directories is strong.
    - [This "what is LDAP" document](https://www.okta.com/au/identity-101/what-is-ldap/) is a nice rundown on LDAP and AD.
    - [Quickstart to mutate AD through LDAP](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/change-windows-active-directory-user-password).
- Various RDBMS protocols.
- TELNET.  
    - Why not?  
    - [This stackoverflow answer](https://stackoverflow.com/questions/15772355/how-to-send-an-http-request-using-telnet) is a nice, simple example.
- DHCP.
- DNS.
- ICMP.

There is no fundamental reason we cannot also address unix sockets.

