# Configuration

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

## Credentials

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
