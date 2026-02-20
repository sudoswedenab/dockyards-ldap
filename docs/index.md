## About

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

## Configuration

This operator relies on the dockyards config map to be present, the config
map to use can be configured via the `--config-map` and `--dockyards-namespace`
command line options.

The config map should contain these values:

| Key                            | Description                                     | Example value                   
| ------------------------------ | ----------------------------------------------- | ---------------------------------
| `ldap.address`                 | The address of the LDAP server.                 | `ldaps://openldap.devault.svc`  
| `ldap.polling-interval`        | How often we should sync.                       | `1m`                            
| `ldap.identity-provider-name`  | Name to prefix resources with.[^1]              | `midgard`                       
| `ldap.organization-base-dn`    | DN where organization groups are queried.       | `OU=Dockyards,DC=example,DC=org`
| `ldap.organization-filter`     | Filter to apply in organization group query.    | `(objectClass=group)`           
| `ldap.user-base-dn`            | DN where member accounts are queried.[^2]       | `OU=Users,DC=example,DC=org`    
| `ldap.secret.credentials`      | Name of the [credentials](#credentials) secret. | `ldap-credentials`              

### Credentials

The secret should look something like this:

```yaml

apiVersion: v1
kind: Secret
type: Opaque
metadata:
  namespace: dockyards-system   # The private namespace configured for dockyards.
  name: ldap-credentials        # The name you want for the secret.
stringData:
  login-dn: cn=admin,dc=example,dc=org # This is the "username".
  password: Foobar2000!

```

[^1]: This should be the same as the user provider name in the OIDC
      for the management cluster in case OIDC has been enabled there.                                  

[^2]: For each organization they are a part of, the member should have
      a `memberOf` attribute containing the DN of the organization group.
      e.g. `memberOf: CN=prod,OU=Dockyards,DC=example,DC=org.`.
      Additionally, the user must have a `sAMAccountName` containing their
      username.


## A note on Backstage TechDocs

Backstage TechDocs builds this site via `mkdocs.yaml`. To make this documentation searchable through Backstage, point TechDocs at this repository and enable the MkDocs generator.
