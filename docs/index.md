# Dockyards LDAP

This component synchronizes organizations and their members through LDAP in
to the management cluster. The dockyards users themselves are created
lazily through the OIDC integration upon first sign in. This component
concerns itself with "cluster admins", the people who have access to
managing clusters etc, and not developers users. Developer user permissions
are synchronized via OIDC in the workload clusters instead. OIDC in workload
clusters is configured through the `authenticationConfig` option when
creating a cluster through the dockyards API, see [kubernetes documentation][authentication-config]
for more details of the `authenticationConfig` options.

The high level flow of this component is:

1. Query LDAP groups which correspond to dockyards organizations.
2. Query for LDAP users that are a part of the above groups.
3. Create organizations/namespaces in the management cluster if needed
4. Create members for the above organizations if needed
5. Remove organizations and members from kubernetes which are
   no longer referred to in the LDAP.

In case the LDAP group is removed for the organization, this component will
try to remove it from kubernetes as well if it can. However, this may fail
in case there are clusters or other resources in the organization which block
the deletion. If that occurs, the cluster administrator will have to delete
those resources manually before the organization is eventually deleted.

[authentication-config]: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration
