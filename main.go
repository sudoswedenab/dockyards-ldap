// Copyright 2025 Sudo Sweden AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ldapv3 "github.com/go-ldap/ldap/v3"

	dyconfig "github.com/sudoswedenab/dockyards-backend/api/config"
	dockyardsv1 "github.com/sudoswedenab/dockyards-backend/api/v1alpha3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KeyLDAPAddress 				dyconfig.Key = "ldap.address" 					// Address of the ldap server. e.g. "ldap://openldap.default.svc"
	KeyLDAPPollingInterval 		dyconfig.Key = "ldap.polling-interval" 			// Interval of which the service will refresh the state. e.g. "1m"
	KeyLDAPIdentityProviderName dyconfig.Key = "ldap.identity-provider-name" 	// Name of the provider which will manage the ldap created resources. (same as OIDC)

	KeyLDAPOrganizationBaseDN 	dyconfig.Key = "ldap.organization-base-dn" 		// Which DN we should look for dockyards organizations in. e.g. "ou=Dockyards,dc=example,dc=org"
	KeyLDAPOrganizationFilter 	dyconfig.Key = "ldap.organization-filter" 		// Filter for dockyards organizations in ldap query. e.g. "(objectClass=group)"
	KeyLDAPUserBaseDN 			dyconfig.Key = "ldap.user-base-dn" 				// Location to search for users. e.g. "OU=Users,OU=Tele2,OU=SE,OU=Resources,DC=corp,DC=tele2,DC=com"
	// NOTE: There is no KeyLDAPUserFilter, it is built dynamically from the organizations that exist

	KeyLDAPSecretCredentials 	dyconfig.Key = "ldap.secret.credentials" 		// Name of the secret containing the login-dn and password
)

func main() {
	var logLevel string
	pflag.StringVar(&logLevel, "log-level", "debug", "log level")

	var configMap string
	pflag.StringVar(&configMap, "config-map", "dockyards-system", "name of application config map")

	var dockyardsSystemNamespace string
	pflag.StringVar(&dockyardsSystemNamespace, "dockyards-namespace", "dockyards-system", "dockyards system namespace")

	pflag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger, err := newLogger(logLevel)
	if err != nil {
        fmt.Printf("error preparing logger: %s", err)
        os.Exit(1)
	}

	slogr := logr.FromSlogHandler(logger.Handler())
	ctrl.SetLogger(slogr)

	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("error getting config", "err", err)

		os.Exit(1)
	}


	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = dockyardsv1.AddToScheme(scheme)

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		logger.Error("error creating manager", "err", err)

		os.Exit(1)
	}

	configManagerOptions := []dyconfig.ConfigManagerOption{
        dyconfig.WithLogger(logger),
    }
    dockyardsConfig, err := dyconfig.NewConfigManager(mgr, client.ObjectKey{Namespace: dockyardsSystemNamespace, Name: configMap}, configManagerOptions...)
    if err != nil {
        logger.Error("could not create config manager", "err", err)

        os.Exit(1)
    }

	err = mgr.Add(&LDAPHandler{
		client: mgr.GetClient(),
		config: dockyardsConfig,
		logger: logger,
		systemNamespace: dockyardsSystemNamespace,
	})
	if err != nil {
		logger.Error("could not add ldap handler to manager", "err", err)

		os.Exit(1)
	}

	err = mgr.Start(ctx)
	if err != nil {
		logger.Error("error running manager", "err", err)

		os.Exit(1)
	}
}

type LDAPHandler struct {
	client client.Client
	config *dyconfig.ConfigManager
	logger *slog.Logger
	systemNamespace string
}

var _ manager.Runnable = &LDAPHandler{}

func (h *LDAPHandler) Start(ctx context.Context) error {
	for {
		config := h.Config(ctx)
		if config == nil {
			h.logger.Warn("could not get config, retrying in 5s")
			time.Sleep(5 * time.Second)
			continue
		}

		h.runOnce(ctx, config)

		select {
		case <-ctx.Done(): return nil
		case <-time.After(config.pollingInterval): continue
		}
	}
}

type Config struct {
	pollingInterval time.Duration

	address string
	providerName string

	organizationBaseDN string
	organizationFilter string
	userBaseDN string

	username string
	password string

	members []dockyardsv1.Member
	orgs []dockyardsv1.Organization

	markedMembers map[string]bool
	markedOrgs map[string]bool
}

func (c *Config) orgName(subject string) string {
	return c.providerName + "-" + subject
}

func (c *Config) memberName(subject string) string {
	return c.providerName + "-" + subject
}

func (c *Config) userName(subject string) string {
	return c.providerName + "-" + subject
}

func (c *Config) markOrg(subject string) {
	c.markedOrgs[c.orgName(subject)] = true
}

func (c *Config) memberMark(orgName string, memberName string) string {
	return orgName + "/" + memberName
}

func (c *Config) markMember(orgSubject string, memberSubject string) {
	mark := c.memberMark(c.orgName(orgSubject), c.memberName(memberSubject))
	c.markedMembers[mark] = true
}

func (c *Config) isMemberMarked(member *dockyardsv1.Member) bool {
	org := member.Labels[dockyardsv1.LabelOrganizationName]
	return c.markedMembers[c.memberMark(org, member.Name)]
}

func (c *Config) resourcesToDelete() []client.Object {
	var result []client.Object

	for i := range c.members {
		if c.isMemberMarked(&c.members[i]) {
			continue
		}
		result = append(result, &c.members[i])
	}

	for _, org := range c.orgs {
		if c.markedOrgs[org.Name] {
			continue
		}
		result = append(result, &org)
	}

	return result
}

func (c *Config) orgExists(subject string) bool {
	name := c.orgName(subject)
	for _, org := range c.orgs {
		if org.Name != name {
			continue
		}
		return true
	}
	return false
}

func (c *Config) memberProviderID(subject string) string {
	return fmt.Sprintf("%s://%s", c.providerName, subject)
}

func (c *Config) orgProviderID(subject string) string {
	return fmt.Sprintf("%s://%s", c.providerName, subject)
}

func (c *Config) memberExists(orgSubject string, memberSubject string) bool {
	orgName := c.orgName(orgSubject)
	memberName := c.memberName(memberSubject)
	for _, member := range c.members {
		if member.Name != memberName {
			continue
		}
		if member.Labels[dockyardsv1.LabelOrganizationName] != orgName {
			continue
		}
		return true
	}
	return false
}

func (h *LDAPHandler) Config(ctx context.Context) *Config {
	didError := false

	pollingIntervalString, ok := h.config.GetValueForKey(KeyLDAPPollingInterval)
	if !ok {
		h.logger.Warn("polling interval has not been set in config map, defaulting to 1m", "key", KeyLDAPPollingInterval)
		pollingIntervalString = "5m"
	}
	pollingInterval, err := time.ParseDuration(pollingIntervalString)
	if err != nil {
		h.logger.Error("could not parse polling interval", "err", err)
		didError = true
	}

	address, ok := h.config.GetValueForKey(KeyLDAPAddress)
	if !ok {
		h.logger.Error("address has not been set in config map", "key", KeyLDAPAddress)
		didError = true
	}
	orgBaseDN, ok := h.config.GetValueForKey(KeyLDAPOrganizationBaseDN)
	if !ok {
		h.logger.Error("org base DN has not been set in config map", "key", KeyLDAPOrganizationBaseDN)
		didError = true
	}
	orgFilter, ok := h.config.GetValueForKey(KeyLDAPOrganizationFilter)
	if !ok {
		h.logger.Error("organization filter has not been set in config map", "key", KeyLDAPOrganizationFilter)
		didError = true
	}
	userBaseDN, ok := h.config.GetValueForKey(KeyLDAPUserBaseDN)
	if !ok {
		h.logger.Error("user base DN has not been set in config map", "key", KeyLDAPUserBaseDN)
		didError = true
	}
	identityProviderName := h.config.GetValueOrDefault(KeyLDAPIdentityProviderName, "")
	if identityProviderName == "" {
		h.logger.Error("provider name has not been set in config map", "key", KeyLDAPIdentityProviderName)
		didError = true
	}
	credentialSecretName, ok := h.config.GetValueForKey(KeyLDAPSecretCredentials)
	if !ok {
		h.logger.Error("credentials secret has not been set in config map", "key", KeyLDAPSecretCredentials)
		didError = true
	}

	var secret corev1.Secret
	if credentialSecretName != "" {
		err = h.client.Get(ctx, client.ObjectKey{Namespace: h.systemNamespace, Name: credentialSecretName}, &secret)
		if err != nil {
			h.logger.Error("could not get credentials secret", "err", err, "namespace", h.systemNamespace, "name", credentialSecretName)
			didError = true
		}
	}

	username, ok := secret.Data["login-dn"]
	if !ok {
		h.logger.Error("login-dn not found in secret", "name", credentialSecretName)
		didError = true
	}
	password, ok := secret.Data["password"]
	if !ok {
		h.logger.Error("password not found in secret", "name", credentialSecretName)
		didError = true
	}

	var orgs dockyardsv1.OrganizationList
	var members dockyardsv1.MemberList
	if identityProviderName != "" {
		err := h.client.List(ctx, &orgs,
			client.MatchingLabels{
				dockyardsv1.LabelProviderName: identityProviderName,
			},
		)
		if err != nil {
			h.logger.Error("could not get orgs", "err", err)
			didError = true
		}

		err = h.client.List(ctx, &members, client.MatchingLabels{
			dockyardsv1.LabelProviderName: identityProviderName,
		})
		if err != nil {
			h.logger.Error("could not list org members", "err", err)
			didError = true
		}
	}

	if didError {
		return nil
	}

	return &Config{
		pollingInterval: pollingInterval,
		address: address,
		organizationBaseDN: orgBaseDN,
		organizationFilter: orgFilter,
		userBaseDN: userBaseDN,
		username: string(username),
		password: string(password),
		providerName: identityProviderName,
		orgs: orgs.Items,
		members: members.Items,
		markedMembers: make(map[string]bool, len(orgs.Items)),
		markedOrgs: make(map[string]bool, len(members.Items)),
	}
}

func (h *LDAPHandler) runOnce(ctx context.Context, config *Config) {
	conn, err := ldapv3.DialURL(config.address)
	if err != nil {
		h.logger.Error("could not dial LDAP", "err", err)
		return
	}
	defer conn.Close()

	err = conn.Bind(config.username, config.password)
	if err != nil {
		h.logger.Error("could not bind LDAP", "err", err)
		return
	}
	defer conn.Unbind()

	h.logger.Info("searching for orgs")
	orgQuery, err := conn.SearchWithPaging(&ldapv3.SearchRequest{
		BaseDN: config.organizationBaseDN,
		Scope: ldapv3.ScopeSingleLevel,
		Filter: config.organizationFilter,
		Attributes: []string{
			"gidNumber",
			"cn",
		},
	}, 1024)
	if err != nil {
		h.logger.Error("could not search for organizations", "err", err, "baseDN", config.organizationBaseDN, "filter", config.organizationFilter)
		return
	}
	h.logger.Info("got orgs", "count", len(orgQuery.Entries))

	orgDNs := make([]string, len(orgQuery.Entries))[:0]
	orgIDByDN := make(map[string]string, len(orgQuery.Entries))
	orgNameByID := make(map[string]string, len(orgQuery.Entries))
	for _, org := range orgQuery.Entries {
		id := org.GetAttributeValue("gidNumber")
		if id == "" {
			h.logger.Error("org did not have a gidNumber attribute", "org", org.DN)
			continue
		}
		_, ok := orgIDByDN[org.DN]
		if ok {
			h.logger.Error("duplicate org id found", "org", org.DN)
			continue
		}
		orgIDByDN[org.DN] = id
		orgDNs = append(orgDNs, org.DN)

		commonName := org.GetAttributeValue("cn")
		if commonName == "" {
			h.logger.Warn("org did not have a cn attribute", "org", org.DN)
			commonName = id
		}
		orgNameByID[id] = commonName
	}

	if len(orgDNs) != 0 {
		filter := ""
		filter += "(|"
		for _, org := range orgDNs {
			filter += "(memberOf=" + org + ")"
		}
		filter += ")"

		userQuery, err := conn.SearchWithPaging(&ldapv3.SearchRequest{
			BaseDN: config.userBaseDN,
			Scope: ldapv3.ScopeSingleLevel,
			Filter: filter,
			Attributes: []string{
				"sAMAccountName",
				"memberOf",
			},
		}, 1024)
		if err != nil {
			h.logger.Error("could not search for members", "err", err, "filter", filter)
			return
		}

		// account name => org name
		userGroups := make(map[string][]string, len(userQuery.Entries))
		groupMentioned := make(map[string]bool, len(userQuery.Entries))

		for _, user := range userQuery.Entries {
			accountName := user.GetAttributeValue("sAMAccountName")
			if accountName == "" {
				h.logger.Warn("user did not have sAMAccountName attribute, ignoring it", "dn", user.DN)
				continue
			}
			_, ok := userGroups[accountName]
			if ok {
				h.logger.Warn("duplicate sAMAccountName found, ignoring it", "dn", user.DN, "sAMAccountName", accountName)
				continue
			}

			groups := user.GetAttributeValues("memberOf")
			if len(groups) == 0 {
				h.logger.Warn("member was not a part of any group", "dn", user.DN)
				continue
			}
			groupIDs := make([]string, len(groups))[:0]
			for _, group := range groups {
				id, ok := orgIDByDN[group]
				if !ok {
					continue // Group is not a dockyards org
				}
				groupMentioned[id] = true
				groupIDs = append(groupIDs, id)
			}
			if len(groupIDs) == 0 {
				continue // User is not part of any dockyards org
			}

			userGroups[accountName] = groupIDs
		}

		for group := range groupMentioned {
			displayName := orgNameByID[group]
			err := h.createOrgIfNeeded(ctx, config, group, displayName)
			if err != nil {
				h.logger.Warn("could not create org", "err", err, "name", group, "displayName", displayName)
				continue
			}
		}

		for member, groups := range userGroups {
			for _, group := range groups {
				err := h.createMemberIfNeeded(ctx, config, group, member)
				if err != nil {
					h.logger.Warn("could not create member", "err", err, "org", group, "name", member)
				}
			}
		}
	}

	h.cleanup(ctx, config)
}

func (h *LDAPHandler) cleanup(ctx context.Context, config *Config) {
	resourcesToDelete := config.resourcesToDelete()
	for _, resource := range resourcesToDelete {
		h.logger.Info("resource was not mentioned in ldap, deleting it", "kind", resource.GetObjectKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		err := h.client.Delete(ctx, resource)
		if err != nil {
			h.logger.Warn("could not delete resource", "err", err, "namespace", resource.GetNamespace(), "name", resource.GetName())
			continue
		}
	}
}

func (h *LDAPHandler) createOrgIfNeeded(ctx context.Context, config *Config, subject string, displayName string) error {
	if config.orgExists(subject) {
		config.markOrg(subject)
		return nil
	}

	name := config.orgName(subject)
	h.logger.Info("creating namespace", "subject", subject, "name", name)
	err := h.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	if err != nil {
		return fmt.Errorf("could not create namespace: %w", err)
	}

	h.logger.Info("creating organization", "subject", subject, "displayName", displayName)
	providerID := config.orgProviderID(subject)
	return h.client.Create(ctx, &dockyardsv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				dockyardsv1.LabelProviderName: config.providerName,
			},
		},
		Spec: dockyardsv1.OrganizationSpec{
			ProviderID: &providerID,
			DisplayName: displayName,

			MemberRefs: nil,
			ProjectRef: nil,
			CredentialRef: nil,

			SkipAutoAssign: false,
			Duration: nil,

			NamespaceRef: &corev1.LocalObjectReference{
				Name: name,
			},
		},
	})
}

func (h *LDAPHandler) createMemberIfNeeded(ctx context.Context, config *Config, orgSubject string, userSubject string) error {
	if config.memberExists(orgSubject, userSubject) {
		config.markMember(orgSubject, userSubject)
		return nil
	}

	h.logger.Info("creating member", "orgSubject", orgSubject, "userSubject", userSubject)
	return h.client.Create(ctx, &dockyardsv1.Member{
		ObjectMeta: metav1.ObjectMeta{
			// FIXME: Org name is not necessarily the same as namespace
			Namespace: config.orgName(orgSubject),
			Name: config.memberName(userSubject),
			Labels: map[string]string{
				dockyardsv1.LabelOrganizationName: config.orgName(orgSubject),
				dockyardsv1.LabelProviderName: config.providerName,
			},
		},
		Spec: dockyardsv1.MemberSpec{
			Role: dockyardsv1.RoleUser,
			UserRef: corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("null"),
				Kind: dockyardsv1.UserKind,
				Name: config.userName(userSubject),
			},
		},
	})
}
func newLogger(logLevel string) (*slog.Logger, error) {
    var level slog.Level
    switch logLevel {
    case "debug":
        level = slog.LevelDebug
    case "info":
        level = slog.LevelInfo
    case "warn":
        level = slog.LevelWarn
    case "error":
        level = slog.LevelError
    default:
        return nil, fmt.Errorf("unknown log level %s", logLevel)
    }

    handlerOptions := slog.HandlerOptions{
        Level: level,
    }

    return slog.New(slog.NewTextHandler(os.Stdout, &handlerOptions)), nil
}
