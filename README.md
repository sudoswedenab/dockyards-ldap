# Usage

```console

Usage of dockyards-ldap:
      --config-map string            name of application config map (default "dockyards-system")
      --dockyards-namespace string   dockyards system namespace (default "dockyards-system")
      --log-level string             log level (default "debug")

```

# Configuration values in application config map:

```yaml

ldap.address: ldap://openldap.default.svc           # The address to the LDAP server.
ldap.polling-interval: 1m                           # How often we should sync.
ldap.identity-provider-name: midgard-lab            # Name to prefix resources with. This should be the same as the user provider in OIDC if it is configured.
ldap.base-dn: ou=Dockyards,dc=example,dc=org        # This is where queries will be done.
ldap.organization-filter: (objectClass=posixGroup)  # The filter to apply to get dockyards organizations. The object is assumed to look like a posixGroup (i.e. it should have a gidNumber and memberUid).
ldap.secret.credentials: ldap-credentials           # Name of the secret containing the authentication credentials.

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
