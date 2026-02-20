## How to build dockyards-ldap

### Prerequisites

- [Git](https://git-scm.com/) is installed
- [Go](https://go.dev/) is installed

### Cross compiling for x86 linux

This binary can be packaged for use in a docker image.

```sh

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dockyards-ldap --ldflags="-s -w"

```

### Compiling native binary

This binary is mostly useful when developing locally.

```sh

go build .

```

## How to deploy dockyards-ldap

1. Make sure the [Dockyards CRDs][dockyards-crds] are installed in your cluster.
2. [Cross compile](#cross-compiling-for-x86-linux) a dockyards-ldap binary for linux.
3. Package the built binary (`dockyards-ldap`) in to a Docker image.
3. Deploy the operator.
4. Grant the operator RBAC access to create/delete `member.dockyards.io`, `organization.dockyards.io` and `namespaces`.
5. Populate the Dockyards config map with the application keys, see [configuration](../index.md#configuration) for more details.

[dockyards-crds]: https://github.com/sudoswedenab/dockyards-backend/tree/main/config/crd
