# Operations Guide

## Deployment

1. Make sure the Dockyards CRDs are installed in your cluster.
2. Build the controller binary (`go build ./...`) and package it into an image.
3. Deploy the operator and grant it RBAC access to create/delete `member.dockyards.io`, `organization.dockyards.io` and `namespaces`.
4. Populate the Dockyards config map with the application keys (`ldap.address`, `ldap.polling-interval`, `ldap.identity-provider-name`, `ldap.organization-base-dn`, `ldap.organization-filter`, `ldap.user-base-dn`, `ldap.secret.credentials`)

## Backstage TechDocs

Backstage TechDocs builds this site via `mkdocs.yml`. Point TechDocs at the repository and enable the MkDocs generator so the `docs/` directory becomes searchable documentation.
