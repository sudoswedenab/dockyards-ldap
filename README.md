# dockyards-ldap

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

## Usage

```console

Usage of dockyards-ldap:
      --config-map string            name of application config map (default "dockyards-system")
      --dockyards-namespace string   dockyards system namespace (default "dockyards-system")
      --log-level string             log level (default "debug")

```

## Configuration values in application config map:

```yaml

ldap.address: ldap://openldap.default.svc                   # The address to the LDAP server.
ldap.polling-interval: 1m                                   # How often we should sync.
ldap.identity-provider-name: midgard-lab                    # Name to prefix resources with. This should be the same as the user provider in OIDC if it is configured.
ldap.organization-base-dn: OU=Dockyards,DC=example,DC=org   # This is where organizations will be queried.
ldap.organization-filter: (objectClass=group)               # The filter to apply to dockyards organizations. The organization must have a gidNumber.
ldap.user-base-dn: ou=Users,DC=example,DC=org               # This is where we search for users. For each organization they are a part of, the member should have a memberOf attribute containing the DN of the organization. e.g. memberOf: CN=prod,OU=Dockyards,DC=example,DC=org. Additionally, the user must have a sAMAccountName containing their username.
ldap.secret.credentials: ldap-credentials                   # Name of the secret containing the authentication credentials.

```

## ldap.secret.credentials secret

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

[authentication-config]: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration
