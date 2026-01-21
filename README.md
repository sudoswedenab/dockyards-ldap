# Usage

```console

Usage of dockyards-ldap:
      --config-map string            name of application config map (default "dockyards-system")
      --dockyards-namespace string   dockyards system namespace (default "dockyards-system")
      --log-level string             log level (default "debug")

```

# Configuration values in application config map:

```yaml

ldap.address: ldap://openldap.default.svc                   # The address to the LDAP server.
ldap.polling-interval: 1m                                   # How often we should sync.
ldap.identity-provider-name: midgard-lab                    # Name to prefix resources with. This should be the same as the user provider in OIDC if it is configured.
ldap.organization-base-dn: OU=Dockyards,DC=example,DC=org   # This is where organizations will be queried.
ldap.organization-filter: (objectClass=group)               # The filter to apply to dockyards organizations. The organization must have a gidNumber.
ldap.user-base-dn: ou=Users,DC=example,DC=org               # This is where we search for users. For each organization they are a part of, the member should have a memberOf attribute containing the DN of the organization. e.g. memberOf: CN=prod,OU=Dockyards,DC=example,DC=org. Additionally, the user must have a sAMAccountName containing their username.
ldap.secret.credentials: ldap-credentials                   # Name of the secret containing the authentication credentials.

```

# ldap.secret.credentials secret

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
